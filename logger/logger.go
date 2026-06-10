package logger

import (
	"log"
	"os"
)

// Экспортируемые переменные логгеров, доступные во всем проекте
var (
	UpdateService          *log.Logger
	DeleteDuplicateService *log.Logger
	Error                  *log.Logger
)

// InitLoggers настраивает и открывает файлы логов
// Функция возвращает массив закрывающих функций, чтобы вызвать их в main через defer
func InitLoggers() []func() error {
	var closers []func() error

	// 1. Настройка общего логгера (UpdateService)
	UpdateServiceLogFile, err := os.OpenFile("update.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл update.log: %v", err)
	}
	closers = append(closers, UpdateServiceLogFile.Close)

	// Создаем изолированный логгер для общих логов
	UpdateService = log.New(UpdateServiceLogFile, "UPDATE: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 2. Настройка общего логгера (DeleteDuplicateService)
	DeleteDuplicateLogFile, err := os.OpenFile("delete_d.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл delete_d.log: %v", err)
	}
	closers = append(closers, DeleteDuplicateLogFile.Close)

	// Создаем изолированный логгер для общих логов
	DeleteDuplicateService = log.New(DeleteDuplicateLogFile, "DELETED: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 3. Настройка логгера для ошибок (Error)
	errFile, err := os.OpenFile("errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл errors.log: %v", err)
	}
	closers = append(closers, errFile.Close)

	// Создаем изолированный логгер для ошибок
	Error = log.New(errFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return closers
}
