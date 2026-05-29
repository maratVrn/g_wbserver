// универсальный оркестратор выполнения задач. Используется для многопоточного выполнения определенной задачи
// с возможностью мягкой остановки и передачи различной служебной информации
package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wbserver/logger"
)

// Сервис запускам управления и остановкой задачей
type TaskRunner struct {
	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	RunningWG  sync.WaitGroup
	isWorking  bool
}

func NewTaskRunner() *TaskRunner {
	return &TaskRunner{}
}

// Возвращает указатель на WaitGroup, чтобы её можно было вызвать из интерфейса
func (r *TaskRunner) GetWaitGroup() *sync.WaitGroup {
	return &r.RunningWG
}

// Проверка занятости
func (r *TaskRunner) IsBusy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isWorking
}

// Ручная отмена
func (r *TaskRunner) CancelExecution() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
		r.cancelFunc = nil
		return true
	}
	return false
}

// Главный универсальный метод запуска пула.
// Он принимает:
// 1. Контекст
// 2. Срез элементов для обработки (используем универсальный generic-тип T)
// 3. Количество воркеров
// 4. Саму функцию обработки (бизнес-логику конкретного сервиса)
// TODO: избавиться от служебных сообщений в консоль и передать ошибки в логи
func RunPool[T any](ctx context.Context, runner *TaskRunner, items []T, workerCount int, workerFunc func(item T)) (bool, bool, error) {
	runner.mu.Lock()
	if runner.isWorking {
		runner.mu.Unlock()
		return false, true, fmt.Errorf("процесс уже выполняется")
	}
	runner.isWorking = true
	runner.mu.Unlock()

	defer func() {
		runner.mu.Lock()
		runner.isWorking = false
		runner.mu.Unlock()
	}()

	runner.RunningWG.Add(1)
	defer runner.RunningWG.Done()

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner.mu.Lock()
	runner.cancelFunc = cancel
	runner.mu.Unlock()

	defer func() {
		runner.mu.Lock()
		runner.cancelFunc = nil
		runner.mu.Unlock()
	}()

	// Создаем канал для задач
	jobs := make(chan T, len(items))
	for _, item := range items {
		jobs <- item
	}
	close(jobs)

	var workersWG sync.WaitGroup
	// Подстчет колличества подзадач
	allSubtaskCount := 0
	subtaskCount := 0
	// Маркеры выполнения задач для принятия решения при завершении глобальной задачи
	noError := true
	noStopTask := true

	for w := 1; w <= workerCount; w++ {
		workersWG.Add(1)
		go func(workerID int) {
			fmt.Println("Запускаем канал")
			defer workersWG.Done()
			for {
				// Двухэтапная проверка контекста, которую мы выстрадали
				if workerCtx.Err() != nil {
					fmt.Printf("Воркер %d останавливается по сигналу контекста\n", workerID)
					noStopTask = false
					return
				}

				select {
				case <-workerCtx.Done():
					fmt.Printf("Воркер %d останавливается по сигналу контекста\n", workerID)
					noStopTask = false
					return
				case item, ok := <-jobs:
					if !ok {
						return
					}
					subtaskCount++
					fmt.Println("Запускаем задачу в воркере", workerID, " задача № ", subtaskCount, item)

					defer func() {
						if r := recover(); r != nil {
							noError = false
							logger.Error.Println("Критическая ошибка (паника) в таблице", item, r)
						}
					}()
					// Вызываем бизнес-логику конкретного сервиса
					workerFunc(item)

					allSubtaskCount += subtaskCount
					// Ограничиваем скорость: принудительный отдых горутины
					time.Sleep(300 * time.Millisecond)
				}
			}
		}(w)
	}

	workersWG.Wait()
	return noError, noStopTask, workerCtx.Err()
}
