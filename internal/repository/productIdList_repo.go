package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wbserver/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProductListRepository struct {
	db *gorm.DB
}

// NewProductListRepository Конструктор
func NewProductListRepository(db *gorm.DB) *ProductListRepository {
	return &ProductListRepository{db: db}
}

// GetItemsFromList Получение всех ID — универсальный метод для любой таблицы productList+ID
func (r *ProductListRepository) GetItemsFromList(listID string) ([]models.ProductListItem, error) {
	var items []models.ProductListItem

	// Динамически формируем имя таблицы
	tableName := fmt.Sprintf("productList%s", listID)

	// Проверяем, существует ли такая таблица в БД, прежде чем делать запрос
	if !r.db.Migrator().HasTable(tableName) {
		return nil, fmt.Errorf("таблица %s не найдена", tableName)
	}

	// Делаем запрос к конкретной таблице через .Table()
	// Используем .Find() чтобы получить все записи
	err := r.db.Table(tableName).Find(&items).Error

	return items, err
}

// GetProductListTableNames Возвращаем список всех таблиц productList в которых есть записи (пропускаем нулевые) для формирования задач
func (r *ProductListRepository) GetProductListTableNames() ([]string, error) {
	var tableNames []string
	// Вариант если нужны все таблицы включая нулевые
	//err := r.db.Table("information_schema.tables").
	//	Where("table_schema = ? AND table_name LIKE ?", "public", "productList%").
	//	Pluck("table_name", &tableNames).Error

	err := r.db.Raw(`
	SELECT relname
	FROM pg_class
	JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
	WHERE pg_namespace.nspname = 'public'
	 AND relname LIKE 'productList%'
	 AND relkind = 'r'
	 AND reltuples > 0
	`).Scan(&tableNames).Error

	return tableNames, err
}

// DeleteIdListFromTable Удаляем записи в таблице tableName по списку deleteIdList используется в UpdateProductListService
func (r *ProductListRepository) DeleteIdListFromTable(ctx context.Context, tableName string, deleteIdList []int) error {
	// Универсальная пустая структура для удаления по ID
	type DynamicProduct struct {
		ID int `gorm:"primaryKey;column:id"`
	}
	const staticTableName = "wb_productIdListAll"

	if len(deleteIdList) == 0 || tableName == "" {
		return nil
	}

	const batchSize = 2000
	totalIDs := len(deleteIdList)

	// Запускаем все удаления внутри одной транзакции в
	// Это ускорит процесс, так как PostgreSQL будет делать commit один раз, а не 100 раз.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := 0; i < totalIDs; i += batchSize {
			end := i + batchSize
			if end > totalIDs {
				end = totalIDs
			}

			currentBatch := deleteIdList[i:end]

			// Удаляем данные из текущей таблицы
			err := tx.Table(tableName).
				Where("id IN ?", currentBatch).
				Delete(&DynamicProduct{}).
				Error

			if err != nil {
				return fmt.Errorf("ошибка удаления батча в таблице %s: %w", tableName, err)
			}

			// Также удаляем эти данные из wb_productIdListAll
			err = tx.Table(staticTableName).
				Where("id IN ?", currentBatch).
				Delete(&DynamicProduct{}).
				Error
			if err != nil {
				return fmt.Errorf("ошибка удаления из таблицы %s на индексе %d: %w", staticTableName, i, err)
			}

		}
		return nil
	})

}

// UpdateInBatches Запрос на обновление данных пачками используется в UpdateProductListService
func (r *ProductListRepository) UpdateInBatches(tableName string, step int, processor func([]models.ProductListItem) ([]models.ProductListItem, error)) error {
	var items []models.ProductListItem

	// Используем FindInBatches
	result := r.db.Table(tableName).FindInBatches(&items, step, func(tx *gorm.DB, batch int) error {
		// Вариант без логирования ошибок в бд
		//result := r.db.Table(tableName).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).FindInBatches(&items, step, func(tx *gorm.DB, batch int) error {

		// 1. Отдаем данные в сервис через processor
		updatedItems, err := processor(items)

		if err != nil {
			fmt.Printf("ОШИБКА Обработка пакета %d: %v\n", batch, err.Error())
			return err // Если сервис вернул ошибку, прерываем всё

		}

		//Сохраняем то, что вернул сервис, обратно в базу
		if err := tx.Table(tableName).Save(&updatedItems).Error; err != nil {
			fmt.Printf("ОШИБКА сохранения пакета %d: %v\n", batch, err.Error())
			return err
		}

		// Возвращаем ошибку, чтобы GORM остановился для отладки !!!
		//return fmt.Errorf("DEBUG_STOP")

		return nil
	})

	return result.Error
}

func (r *ProductListRepository) GetItemByID(listID string, itemID int) (*models.ProductListItem, error) {
	var item models.ProductListItem
	tableName := fmt.Sprintf("productList%s", listID)

	// Проверка таблицы
	if !r.db.Migrator().HasTable(tableName) {
		return nil, fmt.Errorf("таблица %s не найдена", tableName)
	}

	// .First(&item, itemID) — ищем одну запись по первичному ключу (ID)
	fmt.Println(itemID)
	err := r.db.Table(tableName).First(&item, itemID).Error
	if err != nil {
		return nil, err
	}

	return &item, nil
}

// DeleteDuplicateInBatches Запрос на обновление данных пачками используется в DeleteDuplicateService

func (r *ProductListRepository) DeleteDuplicateInBatches(tableName string, step int, processor func(int) error) error {
	var items []models.ProductListItem

	result := r.db.Table(tableName).FindInBatches(&items, step, func(tx *gorm.DB, batch int) error {

		// Собираем ID из текущего батча для вашей функции удаления
		var idList []int
		for _, item := range items {
			idList = append(idList, item.ID)
		}

		// Выполняем логику удаления внутри транзакции батча
		deletedCount, err := r.DeleteDuplicateAndCleanHistory(tx, tableName, idList, true)
		if err != nil {
			return err // Прерываем FindInBatches в случае ошибки
		}

		// Передаем count обратно в сервис через колбэк
		if err := processor(deletedCount); err != nil {
			return err
		}

		return nil
	})

	return result.Error
}

// DeleteDuplicateAndCleanHistory Удаляем дубликаты в таблице по простому принципу -  если в WbProductIDListAll
// не совпадает catalogId или ее вообще нет используется в DeleteDuplicateService
func (r *ProductListRepository) DeleteDuplicateAndCleanHistory(db *gorm.DB, tableName string, idList []int, cleanHistory bool) (int, error) {
	if len(idList) == 0 || tableName == "" {
		return 0, nil
	}

	//  Извлекаем expectedCatalogID из имени таблицы
	catalogIDStr := strings.TrimPrefix(tableName, "productList")
	expectedCatalogID, err := strconv.Atoi(catalogIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid table name format: %w", err)
	}

	deletedCount := 0

	// Запускаем транзакцию через GORM
	err = db.Transaction(func(tx *gorm.DB) error {
		var records []models.WbProductIDListAll

		// Получаем только нужные ID одним запросом (SELECT ... WHERE id IN (...))
		err := tx.Where("id IN ?", idList).Find(&records).Error
		if err != nil {
			return err
		}

		// Переносим полученные данные в мапу для быстрого поиска
		foundIDs := make(map[int]int)
		for _, r := range records {
			foundIDs[r.ID] = r.CatalogID
		}

		// Разделяем ID на те, что нужно удалить, и те, что нужно обновить
		var deleteIDList []int
		var keepIDList []int

		for _, id := range idList {
			catalogID, exists := foundIDs[id]
			if !exists || catalogID != expectedCatalogID {
				deleteIDList = append(deleteIDList, id)
			} else {
				keepIDList = append(keepIDList, id)
			}
		}

		deletedCount = len(deleteIDList)

		// Удаляем неподходящие записи
		if len(deleteIDList) > 0 {

			err = tx.Table(tableName).Where("id IN ?", deleteIDList).Delete(nil).Error
			if err != nil {
				return err
			}
		}
		//
		//// Если обновлять нечего, завершаем транзакцию
		//if len(keepIDList) == 0 {
		//	return nil
		//}
		//
		// Вызываем функцию очистки истории цен она же и завершит транзакцию
		if cleanHistory {
			return r.cleanOldPriceHistory(tx, tableName, keepIDList)
			//return nil
		} else {
			return nil
		}
	})

	return deletedCount, err
}

// cleanOldPriceHistory удаляет из JSON-массива priceHistory записи старше 1 года.
func (r *ProductListRepository) cleanOldPriceHistory(tx *gorm.DB, tableName string, keepIDList []int) error {
	var productsToUpdate []models.ProductListItem

	// Загружаем только нужные нам записи
	err := tx.Table(tableName).Where("id IN ?", keepIDList).Find(&productsToUpdate).Error
	if err != nil {
		return fmt.Errorf("failed to fetch products for history cleaning: %w", err)
	}

	// Граница "1 год назад" от текущего момента
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	for _, product := range productsToUpdate {
		if len(product.PriceHistory) == 0 || string(product.PriceHistory) == "null" {
			continue
		}

		var history []models.PriceHistoryEntry
		if err := json.Unmarshal(product.PriceHistory, &history); err != nil {
			return fmt.Errorf("failed to unmarshal price history for id %d: %w", product.ID, err)
		}

		var filteredHistory []models.PriceHistoryEntry
		var historyChanged = false

		for _, entry := range history {
			entryDate, err := time.Parse("02.01.2006", entry.Date)
			if err != nil {
				// Если дата повреждена, сохраняем её, чтобы не потерять данные
				filteredHistory = append(filteredHistory, entry)
				continue
			}

			if entryDate.After(oneYearAgo) {
				filteredHistory = append(filteredHistory, entry)
			} else {
				historyChanged = true
			}
		}

		// Обновляем запись в БД только если старые элементы действительно были удалены
		if historyChanged {
			newJSON, err := json.Marshal(filteredHistory)
			if err != nil {
				return fmt.Errorf("failed to marshal filtered history for id %d: %w", product.ID, err)
			}

			err = tx.Table(tableName).
				Where("id = ?", product.ID).
				Update("priceHistory", datatypes.JSON(newJSON)).
				Error
			if err != nil {
				return fmt.Errorf("failed to update price history for id %d: %w", product.ID, err)
			}
		}
	}

	return nil
}
