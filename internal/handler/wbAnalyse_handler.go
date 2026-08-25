package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw" // <-- ОБЯЗАТЕЛЬНО ДОБАВЬТЕ ЭТУ СТРОКУ
	"image/png"
	_ "image/png" // Нужно для работы image.Decode
	"net/http"
	"strconv"
	"time"
	"wbserver/internal/models"
	"wbserver/internal/repository"

	"github.com/go-analyze/charts"
	"gorm.io/datatypes"
)

// КаталогHandler содержит зависимость от DB
type WBAnalyseHandler struct {
	repo *repository.WBAnalyseRepository
}

func NewWBAnalyseHandler(repo *repository.WBAnalyseRepository) *WBAnalyseHandler {
	return &WBAnalyseHandler{repo: repo}
}

// FindProductsBySubjectWbId Возвращает список товаров с характеристиками по предмету ВБ (id)
func (h *WBAnalyseHandler) FindProductsBySubjectWbId(w http.ResponseWriter, r *http.Request) {
	subIdStr := r.URL.Query().Get("subId")
	if subIdStr == "" {
		http.Error(w, `{"error": "subId parameter is required"}`, http.StatusBadRequest)
		return
	}
	subId, err := strconv.Atoi(subIdStr)
	if err != nil {
		http.Error(w, `{"error": "invalid id format"}`, http.StatusBadRequest)
		return
	}

	// 1. Шаг первый: Ищем каталоги и группируем по предметам
	subjectsData, err := h.repo.FindCatalogsBySubjectID(subId)
	fmt.Println(subjectsData.Name)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Передаем сгруппированные предметы и собираем ID товаров из динамических таблиц
	finalResults, err := h.repo.GetProductsForSubjects([]models.GroupedSubjectResult{*subjectsData})
	if err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка в GetProductIDsForSubjects: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch product ids"})
		return
	}

	// 3. Дополнительный шаг: Сохранение результата в файл
	subIdStr = subjectsData.Name + "_" + subIdStr
	models.SaveResultsToCSV(subIdStr, finalResults)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(finalResults)

}

// FindCatalogsBySubjectName Получить список каталогов и предметов по запросу
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(subjects)

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
	dailySale, monthlySale, stats := models.CalculateSales(priceHistory)

	//  Маршалим
	dailySaleBytes, err := json.Marshal(dailySale)
	monthlySaleBytes, _ := json.Marshal(monthlySale)
	statsBytes, _ := json.Marshal(stats)
	// 4. Формируем и отдаем красивый ответ клиенту
	response := models.PriceHistoryResponse{
		ProductID:    productID,
		CatalogID:    catalogID,
		PriceHistory: priceHistory,
		DailySale:    datatypes.JSON(dailySaleBytes), // Приведение типа []byte -> datatypes.JSON
		MonthlySale:  datatypes.JSON(monthlySaleBytes),
		Stats:        datatypes.JSON(statsBytes),
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

// ShowProductCharts Отрисовка графиков
func (h *WBAnalyseHandler) ShowProductCharts(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	// 1. Валидация ID товара
	idStr := r.URL.Query().Get("id")
	productID, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Некорректный формат ID товара"})
		return
	}

	// 2. Получаем catalogId
	catalogID, err := h.repo.GetCatalogIDByProductID(productID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 3. Получаем сырой JSON истории цен
	priceHistoryRaw, err := h.repo.GetPriceHistoryFromCatalog(catalogID, productID)
	if err != nil || len(priceHistoryRaw) == 0 || string(priceHistoryRaw) == "null" || string(priceHistoryRaw) == "[]" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 4. Распаковываем JSON истории цен
	var history []HistoryPoint
	if err := json.Unmarshal(priceHistoryRaw, &history); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка обработки истории цен"})
		return
	}

	if len(history) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Товара нет в базе данных"})
		return
	}

	// 5. Вызов вашей функции расчета продаж
	dailySale, monthlySale, _ := models.CalculateSales(priceHistoryRaw)

	// Подготавливаем данные для Графика 1 (Цена) и Графика 2 (Остаток)
	var dateLabels []string
	var prices []float64
	var quantities []float64

	for _, point := range history {
		t, err := time.Parse("02.01.2006", point.Date)
		if err != nil {
			dateLabels = append(dateLabels, point.Date)
		} else {
			dateLabels = append(dateLabels, t.Format("02.01.06"))
		}
		prices = append(prices, point.Price)
		quantities = append(quantities, point.Qty)
	}

	// Подготавливаем массивы для Графика 3 (Продажи по дням)
	var dailyLabels []string
	var dailyVolumes []float64
	for _, d := range dailySale {
		t, err := time.Parse("02.01.2006", d.Date)
		if err == nil {
			dailyLabels = append(dailyLabels, t.Format("02.01.06"))
		} else {
			dailyLabels = append(dailyLabels, d.Date)
		}
		dailyVolumes = append(dailyVolumes, float64(d.Volume))
	}

	// Подготавливаем массивы для Графика 4 (Продажи по месяцам)
	var monthlyLabels []string
	var monthlyVolumes []float64
	for _, m := range monthlySale {
		t, err := time.Parse("2006-01", m.Month)
		if err == nil {
			monthlyLabels = append(monthlyLabels, t.Format("01.06"))
		} else {
			monthlyLabels = append(monthlyLabels, m.Month)
		}
		monthlyVolumes = append(monthlyVolumes, float64(m.Volume))
	}

	// Размеры панелей графиков
	width, height := 1024, 350
	var chartBytes [][]byte

	// 6. Универсальная функция генерации ЛИНЕЙНЫХ графиков, которая у вас ТОЧНО РАБОТАЕТ
	renderSingleLineChart := func(title, legendName string, labels []string, data []float64) error {
		// Используем тот самый метод, который успешно вывел первый график
		opt := charts.NewLineChartOptionWithData([][]float64{data})
		opt.Title = charts.TitleOption{Text: title}

		// Напрямую прокидываем текстовые метки дат
		opt.XAxis.Labels = labels
		opt.XAxis.LabelRotation = 0.5

		opt.Legend = charts.LegendOption{SeriesNames: []string{legendName}}

		p := charts.NewPainter(charts.PainterOptions{Width: width, Height: height})
		if err := p.LineChart(opt); err != nil {
			return err
		}

		b, _ := p.Bytes()
		chartBytes = append(chartBytes, b)
		return nil
	}

	// Генерируем по очереди все 4 графика через проверенный метод
	if err := renderSingleLineChart(fmt.Sprintf("История цен для товара ID %d", productID), "Цена (руб.)", dateLabels, prices); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := renderSingleLineChart("Динамика остатков на складе", "Остаток (шт.)", dateLabels, quantities); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := renderSingleLineChart("Продажи по дням", "Объем продаж (шт.)", dailyLabels, dailyVolumes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := renderSingleLineChart("Продажи по месяцам", "Объем продаж (шт.)", monthlyLabels, monthlyVolumes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 7. СБОРКА И ВЕРТИКАЛЬНАЯ СКЛЕЙКА КАРТИНОК (4 графика)
	combinedImg := image.NewRGBA(image.Rect(0, 0, width, height*4))

	for i, bytesData := range chartBytes {
		img, _, err := image.Decode(bytes.NewReader(bytesData))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка сборки финального изображения"})
			return
		}

		yStart := i * height
		yEnd := yStart + height
		draw.Draw(combinedImg, image.Rect(0, yStart, width, yEnd), img, image.Point{}, draw.Src)
	}

	// 8. ОТПРАВКА СФОРМИРОВАННОГО PNG В БРАУЗЕР
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)

	if err := png.Encode(w, combinedImg); err != nil {
		fmt.Printf("[HTTP ERROR] Ошибка записи PNG: %v\n", err)
	}
}
