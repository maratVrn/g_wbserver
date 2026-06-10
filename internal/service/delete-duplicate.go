package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
	"wbserver/internal/models"
	"wbserver/internal/repository"
	"wbserver/logger"
)

type DeleteDuplicateService struct {
	// Репозитории
	productListRepo *repository.ProductListRepository
	taskRepo        *repository.TaskRepository

	// Оркестратор задач
	Runner *TaskRunner
	// Доп опция - очистка истории (удаляем данные старше чем 1 год)
	cleanHistory bool
}

func NewDeleteDuplicateService(productListRepo *repository.ProductListRepository, taskRepo *repository.TaskRepository, runner *TaskRunner) *DeleteDuplicateService {
	return &DeleteDuplicateService{productListRepo: productListRepo, taskRepo: taskRepo, Runner: runner, cleanHistory: false}
}

// CancelExecution Для соответствия интерфейсу BackgroundService
func (s *DeleteDuplicateService) CancelExecution() bool {
	return s.Runner.CancelExecution()
}

// GetWaitGroup Для соответствия интерфейсу BackgroundService
func (s *DeleteDuplicateService) GetWaitGroup() *sync.WaitGroup {
	return s.Runner.GetWaitGroup()
}

// StartBackgroundDeleteDuplicate Логика запуска сервиса
func (s *DeleteDuplicateService) StartBackgroundDeleteDuplicate(cleanHistory bool) error {
	// 1.проверяем занятость.
	if s.Runner.IsBusy() {
		return fmt.Errorf("процесс удаления дубликатов уже запущен")
	}

	// 2. Получаем данные из базы данных на формирование задачи
	task, unfinishedTables, err := s.taskRepo.GetLatestUnfinishedDeleteDuplicateTask()
	if err != nil {
		return fmt.Errorf("ошибка базы данных: %w", err)
	}

	// 3. Запускаем горутину - она гарантированно защищена от двойного старта, так как IsBusy() вызовется повторно в RunPool синхронно.
	go func() {
		// Передаем context.Background() для фоновой работы
		s.cleanHistory = cleanHistory
		err := s.DeleteDuplicateAll(context.Background(), task, unfinishedTables)
		if err != nil {
			log.Printf("[Background Task] Процесс завершился с ошибкой: %v", err)
			// Здесь обновляем статус задачи в БД на "Ошибка"
			return
		}
		log.Println("[Background Task] Процесс успешно завершен!")
		// Здесь обновляем статус задачи в БД на "Успешно"
	}()

	return nil
}

// DeleteDuplicateAll Базовая многопоточная функция удаления дубликатов и очистки истории-передаем в оркестраторе задач
func (s *DeleteDuplicateService) DeleteDuplicateAll(ctx context.Context, task *models.Task, unfinishedTables []string) error {
	defer func() {
		// recover() ловит панику. Если паники не было, он вернет nil
		if r := recover(); r != nil {
			err := fmt.Errorf("%v", r)
			fmt.Println(err.Error())
		}
	}()
	logger.DeleteDuplicateService.Println("Стартуем DeleteDuplicateAll")
	start := time.Now()
	var mu sync.Mutex // Для безопасного обновления слайса productTasks
	//  Формируем слайс структур для быстрого доступа к состояниям для сохранения
	var dbTasks []models.UpdateTask

	// 2. Распаковываем JSON из task.TaskData в слайс структур
	if err := json.Unmarshal(task.TaskData, &dbTasks); err != nil {
		return fmt.Errorf("ошибка парсинга TaskData: %v", err)
	}
	productTasks := make([]models.UpdateTask, 0, len(dbTasks))
	tableIndexMap := make(map[string]int, len(dbTasks))

	for i, p := range dbTasks {
		productTasks = append(productTasks, models.UpdateTask{
			TableName:    p.TableName,
			TableTaskEnd: p.TableTaskEnd,
		})

		tableIndexMap[p.TableName] = i
	}

	//unfinishedTables = unfinishedTables[0:1] // TODO: ДЛЯ ОТЛАДКИ В РЕАЛЕ В КОММЕНТ
	fmt.Println("Всего нужно обработать таблиц ", len(unfinishedTables))

	// Просто вызываем универсальный пул и передаем туда логику парсинга таблицы!
	allDeleteCount := 0 // Кол-во удаленных строк
	noError := true     // Флаг что не было ошибок
	rNoError, noStopTask, err := RunPool(ctx, s.Runner, unfinishedTables, 5, func(table string) {
		// бизнес-логика удаления
		//crUpdateCount, err := s.imitationProcess(table)
		crDeleteCount, err := s.DeleteDuplicateCurrProductList(table)
		allDeleteCount += crDeleteCount
		// Меняем глобальные флаги чтобы значть что в процессе были ошибки или остановки
		if err == nil {
			mu.Lock()
			if index, found := tableIndexMap[table]; found {
				productTasks[index].TableTaskEnd = true // TODO: ДЛЯ ОТЛАДКИ В РЕАЛЕ ПОМЕНЯТЬ НА TRUE!!
			}

			err2 := s.taskRepo.SaveUpdateProductListProgress(task.ID, productTasks)
			if err2 != nil {
				logger.Error.Println("Ошибка SaveUpdateProductListProgress в таблице", table, err2)
			}

			mu.Unlock()
		} else {
			noError = false
			// произошла ошибка
			logger.Error.Println("Ошибка DeleteDuplicate в таблице", table, err)
		}

	})

	if noError && noStopTask && rNoError {
		fmt.Println("меняем статус задачи на тру")
		err = s.taskRepo.SaveUpdateProductListIsEnd(task.ID) // TODO: ДЛЯ ОТЛАДКИ В РЕАЛЕ убрать комм
	}

	logger.DeleteDuplicateService.Println("Завершили DeleteDuplicate всего удалили", allDeleteCount, "Время выполнения", time.Since(start).Round(100*time.Millisecond))
	fmt.Println("DeleteDuplicate_Workers Завершено")
	fmt.Println("Время выполнения: ", time.Since(start).Round(1*time.Millisecond), " удалили дубликатов", allDeleteCount)
	if !noError || !rNoError {
		fmt.Println("Во время выполнения были ошибки!!!")
	}
	if !noStopTask {
		fmt.Println("Задача была принудительно остановлена")
	}
	return err

}

// DeleteDuplicateCurrProductList Удаление дубликатов и урезание истории выбранной таблицы
func (s *DeleteDuplicateService) DeleteDuplicateCurrProductList(tableName string) (int, error) {
	start := time.Now()
	step := 3000

	allDeleteCount := 0
	// Вызываем репозиторий и передаем ему логику обработки "внутри" анонимной функции
	err := s.productListRepo.DeleteDuplicateInBatches(tableName, step, func(count int) error {
		// Этот код вызывается на каждой итерации FindInBatches
		allDeleteCount += count

		//fmt.Printf("В текущем пакете удалено: %d. Всего удалено: %d\n", count, allDeleteCount)
		return nil
	})
	logMessage := "Время выполнения: " + tableName + "  " + time.Since(start).Round(100*time.Millisecond).String() + "  Всего удалено  " + strconv.Itoa(allDeleteCount)
	fmt.Println(logMessage)
	logger.DeleteDuplicateService.Println(logMessage)

	return allDeleteCount, err

}
