package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	serverURL         = "http://localhost:8080/api/v1/"
	contentTypeHeader = "Content-Type"
	contentTypeJSON   = "application/json"
	requestTimeout    = 5 * time.Second
)

// httpClient - HTTP клиент с таймаутом
var httpClient = &http.Client{
	Timeout: requestTimeout,
}

// Структуры для десериализации ответа сервера
type PricePoint struct {
	Date  string  `json:"d"`  // Дата из вашего JSON (например, "17.10.2025")
	Price float64 `json:"sp"` // Цена (sale price)
	Qty   int     `json:"q"`  // Количество (quantity)
}

type APIResponse struct {
	ProductID    int          `json:"productId"`
	CatalogID    int          `json:"catalogId"`
	PriceHistory []PricePoint `json:"priceHistory"`
}

func main() {
	// Укажите ID товара для теста
	productID := 2778386

	// Формируем URL вашего локального сервера (проверьте порт и префикс /api/v1)
	url := fmt.Sprintf("http://localhost:8080/api/v1/wb_analyse/price-history?id=%d", productID)

	fmt.Printf("[TEST] Отправка запроса на: %s\n", url)

	// Настраиваем HTTP-клиент с таймаутом
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Выполняем GET-запрос
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("[TEST ERROR] Не удалось выполнить запрос: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Проверяем HTTP статус-код
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[TEST ERROR] Сервер вернул ошибку (Статус %d): %s\n", resp.StatusCode, string(body))
		return
	}

	// Читаем сырое тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[TEST ERROR] Не удалось прочитать ответ: %v\n", err)
		return
	}

	// Распаковываем JSON в структуру Go
	var apiData APIResponse
	if err := json.Unmarshal(bodyBytes, &apiData); err != nil {
		fmt.Printf("[TEST ERROR] Ошибка парсинга JSON: %v\n", err)
		// Если структура не совпала, выведем сырой текст, чтобы понять, что пришло
		fmt.Printf("[TEST DEBUG] Сырой ответ от сервера: %s\n", string(bodyBytes))
		return
	}

	// Красивый вывод полученных данных в логи
	fmt.Println("\n================ УСПЕШНЫЙ ОТВЕТ API ================")
	fmt.Printf("ID Товара:   %d\n", apiData.ProductID)
	fmt.Printf("ID Каталога: %d\n", apiData.CatalogID)
	fmt.Println("---------------- История изменения цен: ----------------")

	if len(apiData.PriceHistory) == 0 {
		fmt.Println("История цен пуста ([])")
	} else {
		for i, point := range apiData.PriceHistory {
			fmt.Printf("[%02d] Дата: %10s | Цена: %6.2f | Остаток: %d\n", i+1, point.Date, point.Price, point.Qty)
		}
	}
	fmt.Println("======================================================")
}
