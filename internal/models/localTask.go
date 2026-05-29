package models

import (
	"sync"
)

type WBTask struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type TaskStorage struct {
	Mu    sync.RWMutex
	Tasks map[int]*WBTask
}

// NewTaskStorage — конструктор для инициализации карты
func NewTaskStorage() *TaskStorage {
	return &TaskStorage{
		Tasks: make(map[int]*WBTask),
	}
}
