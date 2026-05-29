package repository

import (
	"fmt"
	"wbserver/internal/models"

	"gorm.io/gorm"
)

type ProductListRepository struct {
	db *gorm.DB
}

// Конструктор
func NewProductListRepository(db *gorm.DB) *ProductListRepository {
	return &ProductListRepository{db: db}
}

// Универсальные методы для любой таблицы productList+ID

// Получение всех ID — универсальный метод для любой таблицы productList+ID

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

// Возвращаем список всех таблиц productList в которых есть записи (пропускаем нулевые) для формирования задач
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

		//2. Сохраняем то, что вернул сервис, обратно в базу
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
