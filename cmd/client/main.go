package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"wbserver/internal/models"
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

// getTask делает запрос к серверу и возвращает объект задачи
func getTask(id int) (*models.WBTask, error) {
	// 1. Формируем URL (убедитесь, что сервер запущен на этом порту)
	url := fmt.Sprintf(serverURL+"tasks/%d", id)

	// 2. Выполняем GET-запрос
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	// 3. Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул ошибку: %s", resp.Status)
	}

	// 4. Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	// 5. Декодируем JSON в структуру WBTask
	var task models.WBTask
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("ошибка при парсинге JSON: %v", err)
	}

	return &task, nil
}

// updateWeather обновляет данные о погоде для указанного города
//func updateWeather(ctx context.Context, city string, weather *models.Weather) (*models.Weather, error) {
//	// Кодируем данные о погоде в JSON
//	jsonData, err := json.Marshal(weather)
//	if err != nil {
//		return nil, fmt.Errorf("кодирование JSON: %w", err)
//	}
//
//	// Создаем PUT-запрос с контекстом
//	req, err := http.NewRequestWithContext(
//		ctx,
//		http.MethodPut,
//		fmt.Sprintf("%s"+weatherAPIPath, serverURL, city),
//		bytes.NewBuffer(jsonData),
//	)
//	if err != nil {
//		return nil, fmt.Errorf("создание PUT-запроса: %w", err)
//	}
//	req.Header.Set(contentTypeHeader, contentTypeJSON)
//
//	// Выполняем запрос
//	resp, err := httpClient.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("выполнение PUT-запроса: %w", err)
//	}
//	defer func() {
//		cerr := resp.Body.Close()
//		if cerr != nil {
//			log.Printf("ошибка закрытия тела ответа: %v\n", cerr)
//			return
//		}
//	}()
//
//	// Читаем тело ответа
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//		return nil, fmt.Errorf("чтение тела ответа: %w", err)
//	}
//
//	if resp.StatusCode != http.StatusOK {
//		return nil, fmt.Errorf("обновление данных о погоде (статус %d): %s", resp.StatusCode, string(body))
//	}
//
//	// Декодируем ответ
//	var updatedWeather models.Weather
//	err = json.Unmarshal(body, &updatedWeather)
//	if err != nil {
//		return nil, fmt.Errorf("декодирование JSON: %w", err)
//	}
//
//	return &updatedWeather, nil
//}

func main() {
	//ctx := context.Background()

	log.Println("=== Тестирование API  ===")
	log.Println("🌦️ Получение данных ...")

	task, err := getTask(1)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	fmt.Printf("Получена задача #%d:\nНазвание: %s\nКонтент: %s\n",
		task.ID, task.Title, task.Content)
}
