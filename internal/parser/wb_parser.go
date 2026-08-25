package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wbserver/internal/models"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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

func (s *WBParserService) ParseProductListTest(idList []int) (map[int]models.WBProductResult, error, int) {
	finalResult := make(map[int]models.WBProductResult, len(idList))

	if len(idList) == 0 {
		return finalResult, nil, 0
	}
	//baseURL := "https://www.wb.ru/__internal/u-card/cards/v4/detail?dest=-1255987&lang=ru&nm="

	var ids []string
	for _, id := range idList {
		if id > 0 {
			ids = append(ids, strconv.Itoa(id))
		}
	}
	//// 2. Соединяем их через ";"
	//url := baseURL + strings.Join(ids, ";")
	// 1. Создаем изолированный контекст Go, так как во внешнем пакете его нет
	baseCtx := context.Background()

	// Настройки для запуска браузера
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		// Если код будет падать или зависать, можно временно раскомментировать строку ниже,
		// чтобы визуально увидеть, что происходит в браузере:
		// chromedp.Flag("headless", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(baseCtx, opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Устанавливаем жесткий таймаут на всю операцию, чтобы функция не зависала
	ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	// Канал для передачи ID пойманного сетевого запроса
	requestIDChan := make(chan network.RequestID, 1)

	// 2. Включаем слушатель сетевых событий (Network Events)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			// Точечно ловим URL, содержащий нужный эндпоинт u-card/cards/v4/detail
			if strings.Contains(ev.Response.URL, "cards/v4/detail") {
				requestIDChan <- ev.RequestID
			}
		}
	})

	// Формируем URL страницы товара для обычного пользователя
	//productPageURL := fmt.Sprintf("https://wildberries.ru", nm)
	productPageURL := "https://www.wildberries.ru/catalog/257476717/detail.aspx"

	// 3. Запускаем сценарий chromedp
	err := chromedp.Run(ctx,
		network.Enable(), // Активируем перехват трафика в Chrome DevTools
		chromedp.Navigate(productPageURL),
		// Ждем появления блока с ценой и кнопками, что гарантирует отработку фонового API
		chromedp.WaitVisible(`.product-page__grid`, chromedp.ByQuery),
	)
	if err != nil {
		fmt.Println("ошибка имитации браузера: %w", err)
	}

	// 4. Забираем тело ответа из пойманного запроса
	select {
	case reqID := <-requestIDChan:
		var body []byte

		// Выполняем внутреннее действие chromedp для чтения памяти ответа
		err := chromedp.Run(ctx, chromedp.ActionFunc(func(actionCtx context.Context) error {
			var err error
			body, err = network.GetResponseBody(reqID).Do(actionCtx)
			return err
		}))

		if err != nil {
			fmt.Println("не удалось прочитать тело API-ответа: %w", err)
		}

		fmt.Println(string(body))

	case <-time.After(15 * time.Second):
		fmt.Println("таймаут: внутренний запрос к API карт не был отправлен сайтом")
	}

	return finalResult, nil, 0
}

func (s *WBParserService) ParseProductList(idList []int) (map[int]models.WBProductResult, error, int) {

	finalResult := make(map[int]models.WBProductResult, len(idList))
	if len(idList) == 0 {
		return finalResult, nil, 0
	}
	baseURL := "https://www.wb.ru/__internal/u-card/cards/v4/detail?dest=-1255987&lang=ru&nm="

	var ids []string
	for _, id := range idList {
		if id > 0 {
			ids = append(ids, strconv.Itoa(id))
		}
	}
	// 2. Соединяем их через ";"
	url := baseURL + strings.Join(ids, ";")
	// 1. Создаем клиент
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	// 2. Создаем запрос (GET)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Ошибка создания запроса: %v\n", err)

	}

	if err != nil {
		return finalResult, err, 0 // TODO: поставить ошибку инициализации соединения
	}

	//req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", s.wbCookie)
	//req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	//req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", "https://www.wildberries.ru/")
	req.Header.Set("Origin", "https://www.wildberries.ru")

	// 4. Выполняем запрос
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)

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

	}

	return finalResult, nil, 0
}
