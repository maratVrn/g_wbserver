package database

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB инициализирует подключение к базе данных
func InitDB() *gorm.DB {
	dsn := "host=localhost user=postgres password=admin dbname=wb_data2 port=5432"

	// 1. Создаем или открываем файл для логов
	file, err := os.OpenFile("gorm_logs.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Не удалось создать файл лога:", err)
	}

	// 2. Настраиваем логгер GORM
	newLogger := logger.New(
		log.New(file, "\r\n", log.LstdFlags), // Пишем в файл с датой и временем
		logger.Config{
			SlowThreshold:             time.Second,  // Порог медленных запросов
			LogLevel:                  logger.Error, // Писать только ошибки (чтобы файл не рос слишком быстро)
			IgnoreRecordNotFoundError: true,         // Не считать "Record Not Found" ошибкой
			Colorful:                  false,        // Выключаем цвета для текстового файла
		},
	)

	// 3. Передаем этот логгер в конфигурацию GORM

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Не удалось получить sql.DB:", err)
	}

	// Настройка пула
	sqlDB.SetMaxOpenConns(25)           // Максимум открытых соединений (чуть больше ваших 20 горутин)
	sqlDB.SetMaxIdleConns(25)           // Сколько соединений держать в памяти про запас
	sqlDB.SetConnMaxLifetime(time.Hour) // Время жизни соединения
	return db
}
