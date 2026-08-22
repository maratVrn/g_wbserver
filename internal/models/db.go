package models

import (
	"time"

	"gorm.io/datatypes"
)

// Weather представляет информацию о погоде для конкретного города

type Task struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskName      string         `gorm:"column:taskName" json:"taskName"`
	IsEnd         bool           `gorm:"column:isEnd" json:"isEnd"`
	StartDateTime string         `gorm:"column:startDateTime" json:"startDateTime"`
	TaskData      datatypes.JSON `gorm:"column:taskData" json:"taskData"`
	TaskResult    datatypes.JSON `gorm:"column:taskResult" json:"taskResult"`
	CreatedAt     time.Time      `gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"column:updatedAt;autoUpdateTime"`
}

func (Task) TableName() string { return "allTasks" }

type UpdateTask struct {
	TableName    string `json:"tableName"`
	TableTaskEnd bool   `json:"tableTaskEnd"`
}

type ProductListItem struct {
	ID            int            `gorm:"primaryKey" json:"id"`
	Price         int            `gorm:"column:price" json:"price"`
	ReviewRating  float64        `gorm:"column:reviewRating" json:"reviewRating"`
	SubjectId     int            `gorm:"column:subjectId" json:"subjectId"`
	BrandId       int            `gorm:"column:brandId" json:"brandId"`
	TotalQuantity int            `gorm:"column:totalQuantity" json:"totalQuantity"`
	PriceHistory  datatypes.JSON `gorm:"column:priceHistory" json:"priceHistory"`
	Discount      float64        `gorm:"column:discount" json:"discount"`
}

type PriceHistoryEntry struct {
	Date  string `json:"d"`
	Price int    `json:"sp"`
	Qty   int    `json:"q"`
}

// WbProductIDListAll
// Структура для чтения данных из wb_productidlistall
type WbProductIDListAll struct {
	ID        int `gorm:"column:id;primaryKey"`
	CatalogID int `gorm:"column:catalogId"`
}

// TableName Указываем GORM точное имя таблицы для структуры WbProductIDListAll
func (WbProductIDListAll) TableName() string {
	return "wb_productIdListAll"
}

// Subject описывает элемент внутри JSON-массива
type Subject struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ParentID   int    `json:"parentId"`
	ParentName string `json:"parentName"`
}

// Промежуточная структура для выгрузки из БД без ошибок маппинга
type RawCatalogRow struct {
	CatalogID  int    `gorm:"column:catalogId"`
	RawJsonStr string `gorm:"column:subjects"` // Выкачиваем JSON как чистый текст
}

// GroupedSubjectResult описывает предмет и список каталогов, где он встретился
type GroupedSubjectResult struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ParentID   int    `json:"parentId"`
	ParentName string `json:"parentName"`
	CatalogIDs []int  `json:"catalogIds"` // Все ID каталогов, где есть этот предмет
}

// SubjectWithProductsResult — итоговая структура, связывающая предмет и его товары
type SubjectWithProductsResult struct {
	SubjectID   int    `json:"subjectId"`
	SubjectName string `json:"subjectName"`
	ProductIDs  []int  `json:"productIds"`
}

type PriceHistoryResponse struct {
	ProductID    int            `json:"productId"`
	CatalogID    int            `json:"catalogId"`
	PriceHistory datatypes.JSON `json:"priceHistory"`
}
