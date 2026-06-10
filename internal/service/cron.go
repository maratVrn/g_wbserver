package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/robfig/cron/v3"
)

// Формируем cron-строку: "минуты часы * * *"
func getCronSpec(startTime string) string {
	parts := strings.Split(startTime, ":")
	if len(parts) != 2 {
		panic("Неверный формат времени")
	}
	hour := parts[0]
	minute := parts[1]
	cronSpec := fmt.Sprintf("%s %s * * *", minute, hour)
	return cronSpec
}

// InitScheduler Создаем планировщик запуска задач
// сначала выполняем updateService
func InitScheduler(updateService *UpdateProductListService /*, loadService *LoadProductsService*/) {

	startTime := "13:51"
	c := cron.New()
	// 1. ЗАПУСК: Каждый день в startTime
	_, err := c.AddFunc(getCronSpec(startTime), func() {
		log.Println("[Scheduler] Время", startTime, "Попытка запуска UpdateProductListService...")

		// сначала запускаем updateService
		err := updateService.StartBackgroundUpdate(false)
		if err != nil {
			log.Printf("[Scheduler] Не удалось запустить сервис: %v", err)
			return
		}

		log.Println("[Scheduler] Сервис успешно запущен в фоновом режиме.")

		//// Запускаем ВТОРОЙ сервис
		//log.Println("[Scheduler] Шаг 2: Запуск LoadProductsService...")
		//err = loadService.LoadAllPages(ctx, urls)
		//if err != nil {
		//	log.Printf("[Scheduler] LoadProductsService завершился с ошибкой или был прерван: %v", err)
		//}
		//
		log.Println("[Scheduler] Ежедневная цепочка сервисов полностью завершена.")
	})
	if err != nil {
		log.Fatalf("Ошибка настройки cron на запуск: %v", err)
	}

	// 2. ОСТАНОВКА: Каждый день в 10:00
	//_, err = c.AddFunc("0 10 * * *", func() {
	//	log.Println("[Scheduler] Время 10:00! Принудительно останавливаем активные сервисы...")
	//
	//	// Отменяем выполнение ОБОИХ сервисов.
	//	// Метод CancelExecution() безопасен: если сервис не запущен, он просто вернет false.
	//	if updateService.Runner.CancelExecution() {
	//		log.Println("[Scheduler] Отправлен сигнал остановки в UpdateProductListService")
	//	}
	//	if loadService.Runner.CancelExecution() {
	//		log.Println("[Scheduler] Отправлен сигнал остановки в LoadProductsService")
	//	}
	//})
	//if err != nil {
	//	log.Fatalf("Ошибка настройки cron на остановку: %v", err)
	//}

	// Запускаем планировщик в фоновом потоке
	c.Start()
	log.Println("[Scheduler] Планировщик задач успешно запущен")
}
