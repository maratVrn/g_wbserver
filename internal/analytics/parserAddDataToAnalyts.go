package analytics

// Парсим товары с WB собираем ткущую инфу
//TODO: Добавить ID предмета и название
//type ProductInfo struct {
//	ID             string
//	Name           string
//	ReviewRating   string
//	Feedbacks      string
//	Price          string
//	Supplier       string
//	SupplierId     string
//	SupplierRating string
//	isInSale       string // Есть ли в продаже
//}

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func ParserAddDataToAnalytics(fileNameOne string) {
	fileName := "analytics\\" + fileNameOne
	//Filtered_Проволока_6006.csv
	logFilename := "analytics\\log\\parser.log"

	// Шаг 1: Принудительно удаляем старый лог-файл перед стартом,
	// чтобы логи не копились годами, а создавались заново за ОДИН запуск программы
	_ = os.Remove(logFilename)

	// Шаг 2: Открываем файл с жестким флагом ДОЗАПИСИ (os.O_APPEND)
	logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf("Критическая ошибка: не удалось инициализировать файл логов: %v\n", err)
		return
	}
	defer logFile.Close()

	// Шаг 3: Объединяем вывод
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	// Тест записи до начала основной логики
	log.Println("[СТАРТ] Инициализация системы логирования успешна.")

	start := time.Now()
	// 1. Загружаем ссылки из файла parsing.txt
	urls, err := readURLsFromFile(fileName)

	if err != nil {
		log.Println("Ошибка загрузки данных из parsing.txt", err)
		return
	}
	outputFile := "analytics\\temp.txt"
	file, err := os.Create(outputFile)
	if err == nil {
		writer := csv.NewWriter(file)
		writer.Comma = ';'
		// Заголовки колонок
		//_ = writer.Write([]string{"ID", "Цена Кошелек", "Цена СПП", "Рейтинг", "Отзывы"})

		_ = writer.Write([]string{"ID", "Name", "Средняя цена", "Средние продажи в месяц",
			"Активных месяцев", "Рейтинг", "Отзывы", "Цена сейчас", "Продавец", "id продавца", "Рейтинг продавца", "В наличии?", "ID предмета"})

		writer.Flush()
		file.Close()
	}
	// 1. Динамически получаем путь к папке, где лежит сам rusUpdate2_main.exe
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	currentDir := filepath.Dir(exePath)

	// 2. Формируем безопасный абсолютный путь к папке профиля внутри текущей директории
	chromeProfilePath := filepath.Join(currentDir, "chrome_profile")
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// Флаг, который разворачивает окно на максимум при старте
		chromedp.Headless,
		// Динамический путь к профилю
		chromedp.UserDataDir(chromeProfilePath),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("headless", false), // ОБЯЗАТЕЛЬНО false для отладки, иначе WB сразу выдаст капчу
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// Используем актуальный User-Agent (минимум Chrome 125+)
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	// 1. Создаем ДОЛГОВЕЧНЫЙ контекст браузера БЕЗ таймаута (он живет, пока идет цикл)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	// Включаем сетевой контур на долговечном контексте
	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		log.Fatal(err)
	}
	// ОБЯЗАТЕЛЬНО: Полностью отключаем кэш браузера, чтобы он каждый раз делал реальный запрос сети
	if err := chromedp.Run(browserCtx, network.SetCacheDisabled(true)); err != nil {
		log.Fatal(err)
	}

	// 1. Создаем глобальные переменные для синхронизации
	var mu sync.Mutex

	// Вместо WaitGroup создаем указатель на канал текущей страницы
	var currentDoneChan chan struct{}
	var jsonResult string
	// Настраиваем слушателя сети ОДИН раз
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Contains(ev.Response.URL, "_internal/u-card") {
				log.Println("нашли u-card")
				go func(reqID network.RequestID) {
					var body []byte
					err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
						var err error
						body, err = network.GetResponseBody(reqID).Do(ctx)
						return err
					}))

					if err == nil {
						mu.Lock()
						// Записываем JSON, только если мы еще не получили данные для этой страницы
						if jsonResult == "" {
							jsonResult = string(body)
							log.Println("Получили json", jsonResult)
							// Безопасно закрываем канал текущей страницы, чтобы разблокировать цикл
							if currentDoneChan != nil {
								// Используем defer+recover на случай, если канал уже закрыт другим потоком
								defer func() { recover() }()
								close(currentDoneChan)
							}
						}
						mu.Unlock()
					}
				}(ev.RequestID)
			}
		}
	})

	// Основной цикл прохода по товарам
	for i, urlObj := range urls {
		log.Println("//**********************************************************************************//")
		log.Println(urlObj.id)
		// Он создается внутри цикла и сбрасывается в конце каждой итерации через defer/вызов cancel
		iterationCtx, cancelIteration := context.WithTimeout(browserCtx, 30*time.Second)

		mu.Lock()
		jsonResult = ""
		// Инициализируем новый чистый канал сигналов для ЭТОЙ конкретной страницы
		currentDoneChan = make(chan struct{})
		mu.Unlock()

		log.Println("переходим на", urlObj.url)
		// Выполняем переход на страницу товара
		err := chromedp.Run(iterationCtx, chromedp.Navigate(urlObj.url))
		if err != nil {
			log.Printf("Ошибка перехода: %v", err)
			cancelIteration()
			continue
		}

		// Линейно ждем ответа, защищаясь от дублирования сетевых запросов
		select {
		case <-currentDoneChan:
			// Канал закрылся фоновой горутиной -> JSON успешно считан
			mu.Lock()
			currentJSON := jsonResult
			mu.Unlock()

			log.Printf("🎉 Успешно перехвачен JSON для товара %d!\n", i+1)
			//fmt.Println(string(currentJSON))
			// Вызываем вашу строковую функцию парсинга
			info, err := ExtractProductInfoOne(currentJSON)
			fmt.Println("info", info)
			if err != nil {
				log.Printf("Ошибка парсинга: %v", err)
				row := []string{urlObj.id, "ERROR", err.Error()}
				saveToCSV(outputFile, row)
			} else {
				log.Printf("Результат -> ID: %s | Имя: %s | Цена: %s\n", info.ID, info.Name, info.Price)

				//{"ID", "Name", "Средняя цена", "Средние продажи в месяц",
				//	"Активных месяцев","Рейтинг", "Отзывы","Цена сейчас","Продавец",
				//	"id продавца","Рейтинг продавца","В наличии?"})

				row := []string{info.ID, info.Name, urlObj.avgPrice, urlObj.avgSalesMonth, urlObj.activeMonths,
					info.ReviewRating, info.Feedbacks, info.Price, info.Supplier, info.SupplierId,
					info.SupplierRating, info.isInSale, info.SubjectId}
				saveToCSV(outputFile, row)

			}

		case <-time.After(30 * time.Second):
			// Если за 12 секунд WB не отдал JSON карточки
			log.Printf("⚠️ Таймаут ожидания ответа для товара %d\n", i+1)
			row := []string{urlObj.id, "ERROR", "Таймаут ожидания ответа"}
			log.Println("Сохраняем данные в cvs")
			saveToCSV(outputFile, row)
		}
		cancelIteration()

		// Делаем паузу между страницами, чтобы сбросить сетевую активность
		time.Sleep(2 * time.Second)
	}

	log.Printf("Время выполнения кода: %v\n", time.Since(start))
	// 2. Переименовываем итоговый файл
	finalFile := "analytics\\results.txt" // Исправили .cvs на .csv

	err = os.Rename(outputFile, finalFile)
	if err != nil {
		log.Printf("Не удалось переименовать файл: %v", err)
	} else {
		log.Printf("Результаты успешно сохранены в: %s", finalFile)
	}
}

// wbUrlData структура со всеми полями из файла аналитики
type wbUrlData struct {
	url           string
	id            string
	avgPrice      string // Средняя цена
	avgSalesMonth string // Средние продажи в месяц
	activeMonths  string // Активных месяцев
}

// ParseOneProduct  принимает сырую строку JSON и возвращает структуру с данными товара
func ParseOneProduct(rawProd wbProductRaw) (ProductInfo, error) {
	// Считаем цену
	var finalPrice float64 = 0
	if len(rawProd.Sizes) > 0 {
		rawPrice := rawProd.Sizes[0].Price.Product
		if rawPrice > 0 {
			finalPrice = float64(rawPrice) / 100.0
		}
	}
	// Формируем чистый результат, включая ID
	crInSale := "Нет"
	if finalPrice > 0 {
		crInSale = "Да"
	}
	info := ProductInfo{
		ID:   fmt.Sprintf("%d", rawProd.ID),
		Name: rawProd.Name,
		// %.1f — для Float, оставляет ровно 1 знак после запятой (например, "4.7")
		ReviewRating:   fmt.Sprintf("%.1f", rawProd.ReviewRating),
		SupplierRating: fmt.Sprintf("%.1f", rawProd.SupplierRating),
		SupplierId:     fmt.Sprintf("%d", rawProd.SupplierId),
		Supplier:       rawProd.Supplier,
		Feedbacks:      fmt.Sprintf("%d", rawProd.Feedbacks),
		// %.2f — для цены, оставляет 2 знака после запятой для копеек (например, "230.00")
		// Если копейки не нужны, замените на "%.0f" (будет просто "230")
		Price:     fmt.Sprintf("%.0f", finalPrice),
		isInSale:  crInSale,
		SubjectId: fmt.Sprintf("%d", rawProd.SubjectId),
	}

	return info, nil
}

// ExtractProductInfo принимает сырую строку JSON и возвращает структуру с данными товара
// Возвращает только первый товар в массив
func ExtractProductInfoOne(jsonStr string) (ProductInfo, error) {
	var target wbResponseRaw
	// Распаковываем сырой JSON
	err := json.Unmarshal([]byte(jsonStr), &target)
	if err != nil {
		return ProductInfo{}, fmt.Errorf("не удалось распарсить JSON: %w", err)
	}
	// Проверяем, пришли ли продукты в массиве
	if len(target.Products) == 0 {
		return ProductInfo{}, fmt.Errorf("массив продуктов в полученном JSON пуст")
	}

	return ParseOneProduct(target.Products[0])
}

type ProductInfo struct {
	ID             string
	Name           string
	ReviewRating   string
	Feedbacks      string
	Price          string
	Supplier       string
	SupplierId     string
	SupplierRating string
	isInSale       string // Есть ли в продаже
	SubjectId      string
}

// Внутренние структуры для маппинга сырого JSON
type wbPriceRaw struct {
	Product int `json:"product"`
}

type wbSizeRaw struct {
	Price wbPriceRaw `json:"price"`
}

type wbProductRaw struct {
	ID             int         `json:"id"`
	Name           string      `json:"name"`
	ReviewRating   float64     `json:"reviewRating"`
	Feedbacks      int         `json:"feedbacks"`
	Sizes          []wbSizeRaw `json:"sizes"` // Добавили массив размеров
	Supplier       string      `json:"supplier"`
	SupplierId     int         `json:"supplierId"`
	SupplierRating float64     `json:"supplierRating"`
	SubjectId      int         `json:"subjectId"`
}

type wbResponseRaw struct {
	Products []wbProductRaw `json:"products"`
}

// saveToCSV записывает строку данных в файл. Использует точку с запятой для совместимости с Excel.
func saveToCSV(filePath string, record []string) error {
	// Открываем файл в режиме: Создать если нет (O_CREATE), Только запись (O_WRONLY), Дописывать в конец (O_APPEND)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';' // Разделитель точка с запятой, чтобы Excel сразу правильно разбивал на колонки
	defer writer.Flush()

	// Записываем строку
	return writer.Write(record)
}

func readURLsFromFile(filePath string) ([]wbUrlData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []wbUrlData

	// Создаем CSV ридер и задаем разделитель точки с запятой
	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Читаем первую строку (заголовок), чтобы пропустить её
	_, err = reader.Read()
	if err != nil {
		if err == io.EOF {
			return urls, nil
		}
		return nil, err
	}

	// Построчно читаем данные
	for {
		record, err := reader.Read()
		fmt.Println(record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// record[0] - ID, record[1] - Средняя цена, record[2] - Средние продажи, record[3] - Активные месяцы
		id := strings.TrimSpace(record[0])

		// Очистка от BOM, если он остался в первом ID после пропуска заголовка (на случай Mac/Windows специфики)
		id = strings.TrimPrefix(id, "\uFEFF")

		if id == "" {
			continue
		}

		url := "https://www.wildberries.ru/catalog/" + id + "/detail.aspx"

		urls = append(urls, wbUrlData{
			url:           url,
			id:            id,
			avgPrice:      strings.TrimSpace(record[1]),
			avgSalesMonth: strings.TrimSpace(record[2]),
			activeMonths:  strings.TrimSpace(record[3]),
		})
	}

	return urls, nil
}
