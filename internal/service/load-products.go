package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"wbserver/internal/models"
	parser2 "wbserver/internal/parser"
	"wbserver/internal/repository"
)

type LoadProductListService struct {
	// Репозитории
	productListRepo *repository.ProductListRepository
	taskRepo        *repository.TaskRepository
	// Сервисы
	parser *parser2.WBParserService
	// Оркестратор задач
	Runner *TaskRunner
	// Удалять товары которые больше не работают на WB
	needDeleteNull bool
}

func NewLoadProductListService(productListRepo *repository.ProductListRepository, taskRepo *repository.TaskRepository, parser *parser2.WBParserService, runner *TaskRunner) *LoadProductListService {
	return &LoadProductListService{productListRepo: productListRepo, taskRepo: taskRepo, parser: parser, Runner: runner, needDeleteNull: false}
}

// CancelExecution Для соответствия интерфейсу BackgroundService
func (s *LoadProductListService) CancelExecution() bool {
	return s.Runner.CancelExecution()
}

// GetWaitGroup Для соответствия интерфейсу BackgroundService
func (s *LoadProductListService) GetWaitGroup() *sync.WaitGroup {
	return s.Runner.GetWaitGroup()
}

// StartBackgroundLoad Логика запуска сервиса
func (s *LoadProductListService) StartBackgroundLoad(needDeleteNull bool) error {
	// 1.проверяем занятость.
	if s.Runner.IsBusy() {
		return fmt.Errorf("процесс загрузки товаров уже запущен")
	}

	// 2. Получаем данные из базы данных на формирование задачи
	task, unfinishedJobs, err := s.taskRepo.GetLatestUnfinishedUpdateTask()
	if err != nil {
		return fmt.Errorf("ошибка базы данных: %w", err)
	}

	// 3. Запускаем горутину - она гарантированно защищена от двойного старта, так как IsBusy() вызовется повторно в RunPool синхронно.
	go func() {
		// Передаем context.Background() для фоновой работы
		s.needDeleteNull = needDeleteNull
		err := s.LoadAllProductList(context.Background(), task, unfinishedJobs)
		if err != nil {
			log.Printf("[Background Task] Процесс завершился с ошибкой: %v", err)
			return
		}
		log.Println("[Background Task] Процесс успешно завершен!")
	}()
	return nil
}

// LoadAllProductList Базовая многопоточная функция обновления таблиц - создаем список задач на обнвовление и передаем в оркестратор задач
func (s *LoadProductListService) LoadAllProductList(ctx context.Context, task *models.Task, unfinishedTables []string) error {
	//defer func() {
	//	// recover() ловит панику. Если паники не было, он вернет nil
	//	if r := recover(); r != nil {
	//		err := fmt.Errorf("%v", r)
	//		fmt.Println(err.Error())
	//	}
	//}()
	//
	//start := time.Now()
	//var mu sync.Mutex // Для безопасного обновления слайса productTasks
	////  Формируем слайс структур для быстрого доступа к состояниям для сохранения
	//var dbTasks []models.UpdateTask
	//
	//// 2. Распаковываем JSON из task.TaskData в слайс структур
	//if err := json.Unmarshal(task.TaskData, &dbTasks); err != nil {
	//	return fmt.Errorf("ошибка парсинга TaskData: %v", err)
	//}
	//productTasks := make([]models.UpdateTask, 0, len(dbTasks))
	//tableIndexMap := make(map[string]int, len(dbTasks))
	//
	//for i, p := range dbTasks {
	//	productTasks = append(productTasks, models.UpdateTask{
	//		TableName:    p.TableName,
	//		TableTaskEnd: p.TableTaskEnd,
	//	})
	//
	//	tableIndexMap[p.TableName] = i
	//}
	//
	////unfinishedTables = unfinishedTables[0:3] // для отладки
	//fmt.Println("Всего нужно обработать таблиц ", len(unfinishedTables))
	//
	//// Просто вызываем универсальный пул и передаем туда логику парсинга таблицы!
	//allUpdateCount := 0 // Общее кол-во сохраненных строк
	//allDeleteCount := 0 // Кол-во удаленных строк
	//noError := true     // Флаг что не было ошибок
	//rNoError, noStopTask, err := RunPool(ctx, s.Runner, unfinishedTables, 5, func(table string) {
	//	// бизнес-логика обновления
	//	//crUpdateCount, err := s.imitationProcess(table)
	//	crUpdateCount, allMissingIDs, err := s.UpdateCurrProductList(table)
	//
	//	allUpdateCount += crUpdateCount
	//	// Меняем глобальные флаги чтобы значть что в процессе были ошибки или остановки
	//	if err == nil {
	//		mu.Lock()
	//		if index, found := tableIndexMap[table]; found {
	//			productTasks[index].TableTaskEnd = true
	//		}
	//
	//		err2 := s.taskRepo.SaveUpdateProductListProgress(task.ID, productTasks)
	//		if err2 != nil {
	//			logger.Error.Println("Ошибка SaveUpdateProductListProgress в таблице", table, err2)
	//		}
	//
	//		mu.Unlock()
	//	} else {
	//		noError = false
	//		// произошла ошибка
	//		logger.Error.Println("Ошибка UpdateCurrProductList в таблице", table, err)
	//	}
	//
	//	if s.needDeleteNull {
	//		fmt.Println("Удаляем все нулевые id", len(allMissingIDs))
	//		allDeleteCount += len(allMissingIDs)
	//		err := s.productListRepo.DeleteIdListFromTable(ctx, table, allMissingIDs)
	//		if err != nil {
	//			noError = false
	//		}
	//	}
	//
	//})
	//
	//if noError && noStopTask && rNoError {
	//	fmt.Println("меняем статус задачи на тру")
	//	err = s.taskRepo.SaveUpdateProductListIsEnd(task.ID)
	//}
	//
	//// TODO:  мы сюда не попадаем если ошибка или стоп контекст и надо убрать лишнее теперь
	//logger.UpdateService.Println("Завершили UpdateAllProductList всего обработано", allUpdateCount, "удалили", allDeleteCount, "Время выполнения", time.Since(start).Round(100*time.Millisecond))
	//fmt.Println("UpdateAllProductList_Workers Завершено")
	//fmt.Println("Время выполнения: ", time.Since(start))
	//if !noError || !rNoError {
	//	fmt.Println("Во время выполнения были ошибки!!!")
	//}
	//if !noStopTask {
	//	fmt.Println("Задача была принудительно остановлена")
	//}
	//return err

	return nil

}
