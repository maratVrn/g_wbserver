package repository

import "gorm.io/gorm"

// Repositories — обертка над всеми репозиториями
type Repositories struct {
	Tasks       *TaskRepository
	ProductList *ProductListRepository
	WBAnalyse   *WBAnalyseRepository
}

func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Tasks:       NewTaskRepository(db),
		ProductList: NewProductListRepository(db),
		WBAnalyse:   NewWBAnalyseRepository(db),
	}
}
