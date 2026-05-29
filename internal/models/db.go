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
