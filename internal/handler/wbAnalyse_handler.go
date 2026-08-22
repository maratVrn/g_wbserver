package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"strconv"
	"time"
	"wbserver/internal/models"
	"wbserver/internal/repository"

	"github.com/go-analyze/charts"
)

// КаталогHandler содержит зависимость от DB
type WBAnalyseHandler struct {
	repo *repository.WBAnalyseRepository
}

func NewWBAnalyseHandler(repo *repository.WBAnalyseRepository) *WBAnalyseHandler {
	return &WBAnalyseHandler{repo: repo}
}

func (h *WBAnalyseHandler) FindCatalogsBySubjectName(w http.ResponseWriter, r *http.Request) {
	searchName := r.URL.Query().Get("searchName")
	if searchName == "" {
		http.Error(w, `{"error": "searchName parameter is required"}`, http.StatusBadRequest)
		return
	}

	// 1. Шаг первый: Ищем каталоги и группируем по предметам
	subjects, err := h.repo.FindCatalogsBySubjectName(searchName)
	if err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка в FindCatalogsBySubjectName: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 2. Шаг второй: Передаем сгруппированные предметы и собираем ID товаров из динамических таблиц
	// Метод GetProductIDsForSubjects должен быть объявлен в вашем h.repo (wbAnalyse_repo.go)
	finalResults, err := h.repo.GetProductsForSubjects(subjects)
	if err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка в GetProductIDsForSubjects: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch product ids"})
		return
	}
	//// 3. Дополнительный шаг: Сохранение результата в файл JSON
	//// Маршалим с отступами, чтобы файл было удобно читать человеку
	//fileData, err := json.MarshalIndent(finalResults, "", "  ")
	//if err != nil {
	//	fmt.Printf("[FILE ERROR] Не удалось перевести результат в JSON для файла: %v\n", err)
	//} else {
	//	// Формируем имя файла, например: search_brelok_20260819_133000.json
	//	timestamp := time.Now().Format("20060102_150405")
	//	fileName := fmt.Sprintf("search_%s_%s.json", searchName, timestamp)
	//
	//	// Записываем байты в файл с правами на чтение/запись
	//	err = os.WriteFile(fileName, fileData, 0644)
	//	if err != nil {
	//		fmt.Printf("[FILE ERROR] Не удалось сохранить файл %s: %v\n", fileName, err)
	//	} else {
	//		fmt.Printf("[FILE SUCCESS] Результаты успешно записаны в файл: %s\n", fileName)
	//	}
	//}
	// 4. Шаг третий: Отдаем клиенту финальный результат с ID товаров
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(finalResults)

}

func (h *WBAnalyseHandler) GetProductPriceHistory(w http.ResponseWriter, r *http.Request) {
	// 1. Получаем параметр id из Query String (?id=93713)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, `{"error": "id parameter is required"}`, http.StatusBadRequest)
		return
	}

	productID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "invalid id format"}`, http.StatusBadRequest)
		return
	}

	// 2. Шаг первый: Идем в wb_productIdListAll и узнаем catalogId
	catalogID, err := h.repo.GetCatalogIDByProductID(productID)
	if err != nil {
		fmt.Printf("[HTTP ERROR] Товар %d не найден в wb_productIdListAll: %v\n", productID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "product or catalog not found"})
		return
	}

	// 3. Шаг второй: Идем в productListXXX и достаем priceHistory
	priceHistory, err := h.repo.GetPriceHistoryFromCatalog(catalogID, productID)
	if err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка чтения истории цен из productList%d: %v\n", catalogID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch price history"})
		return
	}

	// 4. Формируем и отдаем красивый ответ клиенту
	response := models.PriceHistoryResponse{
		ProductID:    productID,
		CatalogID:    catalogID,
		PriceHistory: priceHistory,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Структура для промежуточного парсинга внутренней истории цен
type HistoryPoint struct {
	Date  string  `json:"d"`
	Price float64 `json:"sp"`
	Qty   float64 `json:"q"`
}

func (h *WBAnalyseHandler) ShowProductCharts(w http.ResponseWriter, r *http.Request) {
	// По умолчанию устанавливаем заголовок ответа как JSON для корректного вывода ошибок
	w.Header().Set("Content-Type", "application/json")

	// 1. Получаем и валидируем ID товара из параметров запроса
	idStr := r.URL.Query().Get("id")
	productID, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Некорректный формат ID товара"})
		return
	}

	// 2. Шаг первый: ищем catalogId в wb_productIdListAll
	catalogID, err := h.repo.GetCatalogIDByProductID(productID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 3. Шаг второй: получаем сырой JSON истории цен из productListXXX
	priceHistoryRaw, err := h.repo.GetPriceHistoryFromCatalog(catalogID, productID)

	// Если произошла ошибка БД, либо если массив байт пустой, либо вернулся JSON "null" / "[]"
	if err != nil || len(priceHistoryRaw) == 0 || string(priceHistoryRaw) == "null" || string(priceHistoryRaw) == "[]" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 4. Распаковываем JSON-массив во временную структуру Go
	var history []HistoryPoint
	if err := json.Unmarshal(priceHistoryRaw, &history); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка обработки истории цен"})
		return
	}

	// Дополнительная проверка на пустой слайс после десериализации
	if len(history) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 5. Подготавливаем массивы данных и сокращаем год до формата "ДД.ММ.ГГ"
	var dateLabels []string
	var prices []float64
	var quantities []float64

	for _, point := range history {
		t, err := time.Parse("02.01.2006", point.Date)
		if err != nil {
			// Если дату не удалось распарсить, оставляем оригинальную строку
			dateLabels = append(dateLabels, point.Date)
		} else {
			// Форматируем в компактный вид "10.06.25"
			dateLabels = append(dateLabels, t.Format("02.01.06"))
		}

		prices = append(prices, point.Price)
		quantities = append(quantities, point.Qty)
	}

	// Размеры панелей графиков (ширина и высота каждого)
	width, height := 1024, 350

	// ----------------------------------------------------
	// ГРАФИК 1: ИСТОРИЯ ЦЕН
	// ----------------------------------------------------
	priceOpt := charts.NewLineChartOptionWithData([][]float64{prices})
	priceOpt.Title = charts.TitleOption{
		Text: fmt.Sprintf("История цен для товара ID %d", productID),
	}
	priceOpt.XAxis.Labels = dateLabels
	priceOpt.XAxis.LabelRotation = 0.5 // Поворот дат по диагонали
	//priceOpt.XAxis.SplitNumber = 10     // Ограничение количества подписей на оси X
	priceOpt.Legend = charts.LegendOption{
		SeriesNames: []string{"Цена (руб.)"},
	}

	pricePainter := charts.NewPainter(charts.PainterOptions{Width: width, Height: height})
	if err := pricePainter.LineChart(priceOpt); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Не удалось построить график цен"})
		return
	}
	priceBytes, _ := pricePainter.Bytes()

	// ----------------------------------------------------
	// ГРАФИК 2: ДИНАМИКА ОСТАТКОВ
	// ----------------------------------------------------
	qtyOpt := charts.NewLineChartOptionWithData([][]float64{quantities})
	qtyOpt.Title = charts.TitleOption{
		Text: "Динамика остатков на складе",
	}
	qtyOpt.XAxis.Labels = dateLabels
	qtyOpt.XAxis.LabelRotation = 0.5 // Поворот дат по диагонали
	//qtyOpt.XAxis.SplitNumber = 10     // Ограничение количества подписей на оси X
	qtyOpt.Legend = charts.LegendOption{
		SeriesNames: []string{"Остаток (шт.)"},
	}

	qtyPainter := charts.NewPainter(charts.PainterOptions{Width: width, Height: height})
	if err := qtyPainter.LineChart(qtyOpt); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Не удалось построить график остатков"})
		return
	}
	qtyBytes, _ := qtyPainter.Bytes()

	// ----------------------------------------------------
	// 6. ДЕКОДИРОВАНИЕ И СКЛЕЙКА КАРТИНОК
	// ----------------------------------------------------
	imgPrice, _, err := image.Decode(bytes.NewReader(priceBytes))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка сборки графических данных"})
		return
	}

	imgQty, _, err := image.Decode(bytes.NewReader(qtyBytes))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка сборки графических данных"})
		return
	}

	// Создаем общий холст для вертикальной склейки (высота увеличивается вдвое)
	combinedImg := image.NewRGBA(image.Rect(0, 0, width, height*2))

	// Размещаем: верхний график с y=0, нижний с y=height
	draw.Draw(combinedImg, image.Rect(0, 0, width, height), imgPrice, image.Point{}, draw.Src)
	draw.Draw(combinedImg, image.Rect(0, height, width, height*2), imgQty, image.Point{}, draw.Src)

	// 7. УСПЕШНАЯ ОТПРАВКА ИЗОБРАЖЕНИЯ НА ЭКРАН
	// Переопределяем Content-Type на картинку, так как ошибок нет
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)

	if err := png.Encode(w, combinedImg); err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка записи PNG в http-ответ: %v\n", err)
	}
}
