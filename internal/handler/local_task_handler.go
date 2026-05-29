package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"wbserver/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type LocalTaskHandler struct {
	Storage *models.TaskStorage
}

// GetTask — получение задачи по ID
func (h *LocalTaskHandler) GetLocalTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	h.Storage.Mu.RLock() // Блокировка на чтение
	task, ok := h.Storage.Tasks[id]
	h.Storage.Mu.RUnlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	render.JSON(w, r, task)
}

// PutTask — создание или обновление задачи
func (h *LocalTaskHandler) PutLocalTask(w http.ResponseWriter, r *http.Request) {
	var task models.WBTask

	// Декодируем JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.Storage.Mu.Lock() // Полная блокировка на запись
	h.Storage.Tasks[task.ID] = &task
	h.Storage.Mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, task)
}
