package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"wbserver/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WBAnalyseRepository struct {
	db *gorm.DB
}

// NewWBAnalyseRepository Конструктор
func NewWBAnalyseRepository(db *gorm.DB) *WBAnalyseRepository {
	return &WBAnalyseRepository{db: db}
}

// Вспомогательная функция для вычисления расстояния Левенштейна (кол-во отличий в буквах)
func levenshteinDistance(s, t string) int {
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				min := d[i-1][j] + 1
				if d[i][j-1]+1 < min {
					min = d[i][j-1] + 1
				}
				if d[i-1][j-1]+1 < min {
					min = d[i-1][j-1] + 1
				}
				d[i][j] = min
			}
		}
	}
	return d[len(s)][len(t)]
}

// Функция проверки "похожести" слов
func isSimilar(name, search string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	search = strings.ToLower(strings.TrimSpace(search))

	// 1. Прямое или частичное совпадение (например, "Брелок для ключей" содержит "брелок")
	if strings.Contains(name, search) {
		return true
	}

	// 2. Нечеткий поиск по словам (на случай опечаток: "Брилок", "Брелки")
	words := strings.Fields(name)
	for _, word := range words {
		// Если длина слова сильно отличается от запроса, не тратим время
		if abs(len(word)-len(search)) > 2 {
			continue
		}
		// Если отличие всего в 1-2 буквах, считаем это совпадением
		if levenshteinDistance(word, search) <= 2 {
			return true
		}
	}

	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FindCatalogsBySubjectName Возвращает список похожих предметов и список каталогов где они есть
func (r *WBAnalyseRepository) FindCatalogsBySubjectName(searchName string) ([]models.GroupedSubjectResult, error) {
	var rows []models.RawCatalogRow
	err := r.db.Raw(`SELECT "catalogId", "subjects"::text FROM "AllSubjects"`).Scan(&rows).Error
	if err != nil {
		fmt.Printf("[DEBUG ERROR] Ошибка выгрузки: %v\n", err)
		return nil, err
	}

	// Карта для группировки: ключ — ID предмета, значение — указатель на результат
	groups := make(map[int]*models.GroupedSubjectResult)

	for _, row := range rows {
		var allSubjects []models.Subject
		if err := json.Unmarshal([]byte(row.RawJsonStr), &allSubjects); err != nil {
			continue
		}

		for _, sub := range allSubjects {
			// Проверяем предмет на похожесть
			if isSimilar(sub.Name, searchName) {
				// Если такого предмета еще нет в карте, создаем новую запись
				if _, exists := groups[sub.ID]; !exists {
					groups[sub.ID] = &models.GroupedSubjectResult{
						ID:         sub.ID,
						Name:       sub.Name,
						ParentID:   sub.ParentID,
						ParentName: sub.ParentName,
						CatalogIDs: []int{},
					}
				}
				// Добавляем текущий catalogId в массив этого предмета
				groups[sub.ID].CatalogIDs = append(groups[sub.ID].CatalogIDs, row.CatalogID)
			}
		}
	}

	// Преобразуем карту в плоский массив для JSON-ответа
	var finalResults []models.GroupedSubjectResult
	for _, result := range groups {
		finalResults = append(finalResults, *result)
	}

	fmt.Printf("[DEBUG SUCCESS] Сгруппировано уникальных предметов: %d\n", len(finalResults))
	return finalResults, nil
}

// GetProductsForSubjects собирает товары из таблиц productListXXX по совпадению subjectId

func (r *WBAnalyseRepository) GetProductsForSubjects(subjects []models.GroupedSubjectResult) ([]models.SubjectWithProductsResult, error) {
	var finalResults []models.SubjectWithProductsResult

	// 1. Итерируемся по сгруппированным предметам
	for _, sub := range subjects {
		var allIDsForSubject []int

		// 2. Опрашиваем связанные таблицы каталогов
		for _, catalogID := range sub.CatalogIDs {
			tableName := fmt.Sprintf("productList%d", catalogID)

			var batchIDs []int

			// Запрашиваем ТОЛЬКО колонку id, фильтруя по subjectId
			err := r.db.Raw(fmt.Sprintf(`
				SELECT id 
				FROM "%s" 
				WHERE "subjectId" = ?
			`, tableName), sub.ID).Scan(&batchIDs).Error

			if err != nil {
				// Если таблицы нет или она пуста — просто пропускаем
				fmt.Printf("[DEBUG WARN] Ошибка чтения ID из таблицы %s: %v\n", tableName, err)
				continue
			}

			// Объединяем ID товаров в один большой массив для этого предмета
			allIDsForSubject = append(allIDsForSubject, batchIDs...)
		}

		// 3. Записываем результат для текущего предмета
		finalResults = append(finalResults, models.SubjectWithProductsResult{
			SubjectID:   sub.ID,
			SubjectName: sub.Name,
			ProductIDs:  allIDsForSubject,
		})
	}

	return finalResults, nil
}

// GetCatalogIDByProductID находит catalogId для переданного ID товара
func (r *WBAnalyseRepository) GetCatalogIDByProductID(productID int) (int, error) {
	var catalogID int

	// Важно: оборачиваем "catalogId" в двойные кавычки из-за заглавной буквы
	err := r.db.Raw(`
		SELECT "catalogId" 
		FROM "wb_productIdListAll" 
		WHERE id = ? 
		LIMIT 1
	`, productID).Scan(&catalogID).Error

	if err != nil {
		return 0, err
	}
	return catalogID, nil
}

// GetPriceHistoryFromCatalog вытаскивает priceHistory из динамической таблицы productListXXX
func (r *WBAnalyseRepository) GetPriceHistoryFromCatalog(catalogID int, productID int) (datatypes.JSON, error) {
	// Создаем временную структуру, чтобы GORM знал, какие поля мы ищем и какого они типа
	var result struct {
		PriceHistory datatypes.JSON `gorm:"column:priceHistory"`
	}

	// Динамически задаем имя таблицы через .Table()
	tableName := fmt.Sprintf("productList%d", catalogID)

	// Пишем чистый запрос на GORM без сырого SQL
	err := r.db.Table(tableName).
		Select(`"priceHistory"`). // Берем в кавычки для Postgres
		Where("id = ?", productID).
		Limit(1).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	// Если история цен пустая, возвращаем пустой JSON-массив
	if len(result.PriceHistory) == 0 {
		return datatypes.JSON([]byte("[]")), nil
	}

	return result.PriceHistory, nil
}
