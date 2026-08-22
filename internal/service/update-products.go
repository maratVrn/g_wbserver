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
	parser2 "wbserver/internal/parser"
	"wbserver/internal/repository"
	"wbserver/logger"

	"gorm.io/datatypes"
)

type UpdateProductListService struct {
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

func NewUpdateProductListService(productListRepo *repository.ProductListRepository, taskRepo *repository.TaskRepository, parser *parser2.WBParserService, runner *TaskRunner) *UpdateProductListService {
	return &UpdateProductListService{productListRepo: productListRepo, taskRepo: taskRepo, parser: parser, Runner: runner, needDeleteNull: false}
}

// CancelExecution Для соответствия интерфейсу BackgroundService
func (s *UpdateProductListService) CancelExecution() bool {
	return s.Runner.CancelExecution()
}

// GetWaitGroup Для соответствия интерфейсу BackgroundService
func (s *UpdateProductListService) GetWaitGroup() *sync.WaitGroup {
	return s.Runner.GetWaitGroup()
}

// Перезаписываем данные из полученных
func setNewData(batch []models.ProductListItem, wbParseResult map[int]models.WBProductResult) ([]int, int) {
	var missingIDs []int
	for i := range batch {
		id := batch[i].ID
		if data, ok := wbParseResult[id]; ok {
			if data.Price > 0 { // Цена НЕ нулевая значит товар в наличии
				batch[i].Price = data.Price
				batch[i].ReviewRating = data.ReviewRating
				batch[i].TotalQuantity = data.TotalQuantity

				// 2. Работаем с историей цен
				var history []models.PriceHistoryEntry
				// Распаковываем текущий JSON из базы в слайс history
				if len(batch[i].PriceHistory) > 0 {
					_ = json.Unmarshal(batch[i].PriceHistory, &history)
				}
				// 3. Создаем новую запись
				newEntry := models.PriceHistoryEntry{
					Date:  time.Now().Format("02.01.2006"), // Формат dd.mm.yyyy
					Price: data.Price,
					Qty:   data.TotalQuantity,
				}
				// 4. Добавляем в массив и упаковываем обратно
				today := time.Now().Format("02.01.2006")
				// Если последняя запись уже за сегодня — заменяем её, а не добавляем новую
				needAdd := true
				if len(history) > 0 {
					if history[len(history)-1].Date == today {
						history[len(history)-1].Price = data.Price
						history[len(history)-1].Qty = data.TotalQuantity
						needAdd = false
						// Если же цена совпадает, то ничего не меняем
					} else if history[len(history)-1].Price == data.Price {
						needAdd = false
					}
				}

				if needAdd {
					history = append(history, newEntry)
				}

				newJSON, _ := json.Marshal(history)
				batch[i].PriceHistory = datatypes.JSON(newJSON)

			} else { // товара нет
				batch[i].TotalQuantity = data.TotalQuantity
			}

		} else {
			missingIDs = append(missingIDs, id)
		}
	}
	return missingIDs, 0
}

// UpdateCurrProductList Обновление выбранной таблицы
func (s *UpdateProductListService) UpdateCurrProductList(tableName string) (int, []int, error) {
	start := time.Now()
	step := 500
	allBatchCount := 0
	allResultCount := 0
	allMissingCount := 0
	var allMissingIDs []int

	// Вызываем репозиторий и передаем ему логику обработки "внутри" анонимной функции
	err := s.productListRepo.UpdateInBatches(tableName, step, func(batch []models.ProductListItem) ([]models.ProductListItem, error) {
		// Здесь логика изменения:
		lenBatch := len(batch)
		//fmt.Println("получили batch", lenBatch)

		idList := make([]int, len(batch))
		for i, item := range batch {
			idList[i] = item.ID
		}

		wbParseResult, err, errorCode := s.parser.ParseProductList(idList)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга: %w, код: %d", err, errorCode)
		}

		// TODO: обработать ошибку
		missingIDs, _ := setNewData(batch, wbParseResult)
		if s.needDeleteNull {
			allMissingIDs = append(allMissingIDs, missingIDs...)
		}

		// Для отладки сохраняем список ID на удаление и проверяем в логах
		//logMessage := ""
		//for _, id := range missingIDs {
		//	logMessage += strconv.Itoa(id) + ";"
		//}
		//logger.UpdateService.Println(logMessage)

		lenResult := len(wbParseResult)
		lenMissing := len(missingIDs)
		//fmt.Println("Получили данных", lenResult, "пропущено значений", lenMissing)

		allBatchCount += lenBatch
		allResultCount += lenResult
		allMissingCount += lenMissing

		return batch, nil // Возвращаем измененный батч в репозиторий для сохранения
	})

	logMessage := "Время выполнения: " + tableName + "  " + time.Since(start).Round(100*time.Millisecond).String() + "  Всего обработано  " + strconv.Itoa(allBatchCount) + "  получено  " +
		strconv.Itoa(allResultCount) + " пропуск  " + strconv.Itoa(allMissingCount) + "  проверка  " + strconv.Itoa(allBatchCount-allResultCount-allMissingCount)

	logger.UpdateService.Println(logMessage)

	return allBatchCount, allMissingIDs, err

}

// Заглушка
func (s *UpdateProductListService) imitationProcess(tableName string) (int, error) {
	fmt.Println("Запустили задачу по таблице", tableName)
	time.Sleep(5 * time.Second)
	fmt.Println("Завершили задачу по таблице", tableName)
	return 10, nil

}

// UpdateAllProductList Базовая многопоточная функция обновления таблиц - создаем список задач на обнвовление и передаем в оркестратор задач
func (s *UpdateProductListService) UpdateAllProductList(ctx context.Context, task *models.Task, unfinishedTables []string) error {
	defer func() {
		// recover() ловит панику. Если паники не было, он вернет nil
		if r := recover(); r != nil {
			err := fmt.Errorf("%v", r)
			fmt.Println(err.Error())
		}
	}()

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

	//unfinishedTables = unfinishedTables[0:3] // для отладки
	fmt.Println("Всего нужно обработать таблиц ", len(unfinishedTables))

	// Просто вызываем универсальный пул и передаем туда логику парсинга таблицы!
	allUpdateCount := 0 // Общее кол-во сохраненных строк
	allDeleteCount := 0 // Кол-во удаленных строк
	noError := true     // Флаг что не было ошибок
	rNoError, noStopTask, err := RunPool(ctx, s.Runner, unfinishedTables, 5, func(table string) {
		// бизнес-логика обновления
		//crUpdateCount, err := s.imitationProcess(table)
		crUpdateCount, allMissingIDs, err := s.UpdateCurrProductList(table)

		allUpdateCount += crUpdateCount
		// Меняем глобальные флаги чтобы значть что в процессе были ошибки или остановки
		if err == nil {
			mu.Lock()
			if index, found := tableIndexMap[table]; found {
				productTasks[index].TableTaskEnd = true
			}

			err2 := s.taskRepo.SaveUpdateProductListProgress(task.ID, productTasks)
			if err2 != nil {
				logger.Error.Println("Ошибка SaveUpdateProductListProgress в таблице", table, err2)
			}

			mu.Unlock()
		} else {
			noError = false
			// произошла ошибка
			logger.Error.Println("Ошибка UpdateCurrProductList в таблице", table, err)
		}

		if s.needDeleteNull {
			fmt.Println("Удаляем все нулевые id", len(allMissingIDs))
			allDeleteCount += len(allMissingIDs)
			err := s.productListRepo.DeleteIdListFromTable(ctx, table, allMissingIDs)
			if err != nil {
				noError = false
			}
		}

	})

	if noError && noStopTask && rNoError {
		fmt.Println("меняем статус задачи на тру")
		err = s.taskRepo.SaveUpdateProductListIsEnd(task.ID)
	}

	// TODO:  мы сюда не попадаем если ошибка или стоп контекст и надо убрать лишнее теперь
	logger.UpdateService.Println("Завершили UpdateAllProductList всего обработано", allUpdateCount, "удалили", allDeleteCount, "Время выполнения", time.Since(start).Round(100*time.Millisecond))
	fmt.Println("UpdateAllProductList_Workers Завершено")
	fmt.Println("Время выполнения: ", time.Since(start))
	if !noError || !rNoError {
		fmt.Println("Во время выполнения были ошибки!!!")
	}
	if !noStopTask {
		fmt.Println("Задача была принудительно остановлена")
	}
	return err

}

// StartBackgroundUpdate Логика запуска сервиса
func (s *UpdateProductListService) StartBackgroundUpdate(needDeleteNull bool) error {
	// 1.проверяем занятость.
	if s.Runner.IsBusy() {
		return fmt.Errorf("процесс обновления уже запущен")
	}

	// 2. Получаем данные из базы данных на формирование задачи
	task, unfinishedTables, err := s.taskRepo.GetLatestUnfinishedUpdateTask()
	if err != nil {
		return fmt.Errorf("ошибка базы данных: %w", err)
	}

	// 3. Запускаем горутину - она гарантированно защищена от двойного старта, так как IsBusy() вызовется повторно в RunPool синхронно.
	go func() {
		// Передаем context.Background() для фоновой работы
		s.needDeleteNull = needDeleteNull
		err := s.UpdateAllProductList(context.Background(), task, unfinishedTables)
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
