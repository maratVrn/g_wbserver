package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"wbserver/internal/models"
)

type WBParserService struct {
	wbCookie string
	// Здесь можно добавить зависимости, например, HttpClient или Logger
}

func NewWBParserService(wbCookie string) *WBParserService {
	return &WBParserService{wbCookie: wbCookie}
}

// ParseProduct — пример функции парсинга, getDataFromJsonWB - вспомогательная функция
type ResponseJSON struct {
	Products []struct {
		Id            int     `json:"id"`
		ReviewRating  float64 `json:"reviewRating"`
		TotalQuantity int     `json:"totalQuantity"`
		Sizes         []struct {
			Price struct {
				Product int `json:"product"`
			} `json:"price"`
		} `json:"sizes"`
	} `json:"products"`
}

func (s *WBParserService) ParseProductList(idList []int) (map[int]models.WBProductResult, error, int) {

	finalResult := make(map[int]models.WBProductResult, len(idList))
	if len(idList) == 0 {
		return finalResult, nil, 0
	}
	baseURL := "https://www.wildberries.ru/__internal/u-card/cards/v4/detail?dest=-1255987&lang=ru&nm="

	var ids []string
	for _, id := range idList {
		if id > 0 {
			ids = append(ids, strconv.Itoa(id))
		}
	}
	// 2. Соединяем их через ";"
	url := baseURL + strings.Join(ids, ";")
	// 1. Создаем клиент
	client := &http.Client{}
	// 2. Создаем запрос (GET)
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return finalResult, err, 0 // TODO: поставить ошибку инициализации соединения
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", s.wbCookie)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// 4. Выполняем запрос
	resp, err := client.Do(req)

	if err != nil {
		return finalResult, err, resp.StatusCode
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return finalResult, fmt.Errorf("ошибка запроса парсера WB: статус %d", resp.StatusCode), resp.StatusCode
	}

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return finalResult, err, 0 // TODO: поставить ошибку инициализации соединения
	}

	// Распаковываем JSON в универсальный интерфейс (или структуру, если она у вас есть)
	var res ResponseJSON
	if err := json.Unmarshal(body, &res); err != nil {
		return finalResult, err, resp.StatusCode
	}

	for _, p := range res.Products {
		firstPrice := 0

		// Ищем первую ненулевую цену в массиве sizes
		for _, s := range p.Sizes {
			if s.Price.Product != 0 {
				firstPrice = s.Price.Product / 100
				break // Нашли цену — выходим из внутреннего цикла по sizes
			}
		}

		// Добавляем очищенные данные в итоговый слайс
		finalResult[p.Id] = models.WBProductResult{
			ID:            p.Id,
			ReviewRating:  p.ReviewRating,
			TotalQuantity: p.TotalQuantity,
			Price:         firstPrice,
		}
		//finalResult.Products = append(finalResult.Products, models.WBProductResult{
		//	ID:            p.Id,
		//	ReviewRating:  p.ReviewRating,
		//	TotalQuantity: p.TotalQuantity,
		//	Price:         firstPrice,
		//})
	}

	return finalResult, nil, 0
}
