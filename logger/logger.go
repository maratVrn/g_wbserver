package logger

import (
	"log"
	"os"
	"path/filepath"
)

// Экспортируемые переменные логгеров, доступные во всем проекте
var (
	UpdateService          *log.Logger
	DeleteDuplicateService *log.Logger
	Error                  *log.Logger
)

// LogDir определяет общую папку для логов
const LogDir = "logs"

// AllowedLogFiles — единый источник имен для всех файлов логов.
// Значение true используется для удобной проверки в map.
var AllowedLogFiles = map[string]bool{
	"update.log":   true,
	"delete_d.log": true,
	"errors.log":   true,
}

// InitLoggers настраивает и открывает файлы логов
// Функция возвращает массив закрывающих функций, чтобы вызвать их в main через defer
func InitLoggers() []func() error {
	var closers []func() error

	if err := os.MkdirAll(LogDir, 0755); err != nil {
		log.Fatalf("Не удалось создать директорию для логов: %v", err)
	}
	// Настройка общего логгера (UpdateService)
	updateLogPath := filepath.Join(LogDir, "update.log")
	UpdateServiceLogFile, err := os.OpenFile(updateLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл update.log: %v", err)
	}
	closers = append(closers, UpdateServiceLogFile.Close)

	UpdateService = log.New(UpdateServiceLogFile, "UPDATE: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 2. Настройка общего логгера (DeleteDuplicateService)
	deleteLogPath := filepath.Join(LogDir, "delete_d.log")
	DeleteDuplicateLogFile, err := os.OpenFile(deleteLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл delete_d.log: %v", err)
	}
	closers = append(closers, DeleteDuplicateLogFile.Close)

	DeleteDuplicateService = log.New(DeleteDuplicateLogFile, "DELETED: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Настройка логгера для ошибок (Error)
	errorsLogPath := filepath.Join(LogDir, "errors.log")
	errFile, err := os.OpenFile(errorsLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть файл errors.log: %v", err)
	}
	closers = append(closers, errFile.Close)
	Error = log.New(errFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return closers
}
