package service

// Интерфейс для объединения всех сервисов в один слайс
import "sync"

type BackgroundService interface {
	// Метод для ручной/программной остановки
	CancelExecution() bool

	// Метод, возвращающий WaitGroup конкретного сервиса, чтобы main мог её подождать
	GetWaitGroup() *sync.WaitGroup
}
