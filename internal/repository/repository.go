package repository

import "gorm.io/gorm"

// Repositories — обертка над всеми репозиториями
type Repositories struct {
	Tasks       *TaskRepository
	ProductList *ProductListRepository
}

func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Tasks:       NewTaskRepository(db),
		ProductList: NewProductListRepository(db),
	}
}
