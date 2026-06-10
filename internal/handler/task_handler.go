package handler

import (
	"encoding/json"
	"net/http"
	"wbserver/internal/repository"
	"wbserver/internal/service"

	"github.com/go-chi/chi/v5"
)

type TaskHandler struct {
	repo                   *repository.TaskRepository
	productListRepo        *repository.ProductListRepository
	updateService          *service.UpdateProductListService
	deleteDuplicateService *service.DeleteDuplicateService
}

// TODO: передать логи в отдельный файл задач
func NewTaskHandler(repo *repository.TaskRepository, productListRepo *repository.ProductListRepository, updateService *service.UpdateProductListService,
	deleteDuplicateService *service.DeleteDuplicateService) *TaskHandler {
	return &TaskHandler{repo: repo, productListRepo: productListRepo, updateService: updateService, deleteDuplicateService: deleteDuplicateService}
}

// GetTask TODO:  Получить задачу по ID вроде не испольщуется
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	// Достаем ID из URL: /tasks/{id}
	id := chi.URLParam(r, "id")

	task, err := h.repo.FindByID(id)
	if err != nil {
		http.Error(w, "Задача не найдена", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// CancelUpdateProductList Остановить UpdateAllProductList
func (h *TaskHandler) CancelUpdateProductList(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	//  Проверяем, запущен ли процесс прямо сейчас
	isBusy := h.updateService.Runner.IsBusy()

	if !isBusy {
		w.WriteHeader(http.StatusBadRequest) // 400 Bad Request
		w.Write([]byte(`{"error": "Нет активных процессов обновления для остановки"}`))
		return
	}

	// Если процесс идет, вызываем отмену контекста воркеров
	h.updateService.Runner.CancelExecution()

	// Отдаем успешный ответ
	w.WriteHeader(http.StatusOK) // 200 OK
	w.Write([]byte(`{"status": "Сигнал остановки отправлен. Текущие таблицы дорабатывают и воркеры завершат работу."}`))
}

// UpdateAllProductList Глобальная задача - обновление всех ИД продуктов
func (h *TaskHandler) UpdateAllProductList(w http.ResponseWriter, r *http.Request) {

	err := h.updateService.StartBackgroundUpdate(true)
	if err != nil {
		// Если сервис вернул ошибку, проверяем её тип
		if err.Error() == "процесс обновления уже запущен" {
			w.WriteHeader(http.StatusConflict) // 409
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Любая другая ошибка (например, БД)
		w.WriteHeader(http.StatusInternalServerError) // 500
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Успешный ответ клиенту
	w.WriteHeader(http.StatusAccepted) // 202
	json.NewEncoder(w).Encode(map[string]string{"status": "процесс обновления успешно запущен в фоне"})

}

// DeleteDuplicate Глобальная задача - обновление всех ИД продуктов
func (h *TaskHandler) DeleteDuplicate(w http.ResponseWriter, r *http.Request) {

	err := h.deleteDuplicateService.StartBackgroundDeleteDuplicate(false)
	if err != nil {
		// Если сервис вернул ошибку, проверяем её тип
		if err.Error() == "процесс удаления дубликатов уже запущен" {
			w.WriteHeader(http.StatusConflict) // 409
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Любая другая ошибка (например, БД)
		w.WriteHeader(http.StatusInternalServerError) // 500
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Успешный ответ клиенту
	w.WriteHeader(http.StatusAccepted) // 202
	json.NewEncoder(w).Encode(map[string]string{"status": "процесс удаления дубликатов успешно запущен в фоне"})

}

// CancelDeleteDuplicate Остановить CancelDeleteDuplicate
func (h *TaskHandler) CancelDeleteDuplicate(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	//  Проверяем, запущен ли процесс прямо сейчас
	isBusy := h.deleteDuplicateService.Runner.IsBusy()

	if !isBusy {
		w.WriteHeader(http.StatusBadRequest) // 400 Bad Request
		w.Write([]byte(`{"error": "Нет активных процессов удаления дубликатов"}`))
		return
	}

	// Если процесс идет, вызываем отмену контекста воркеров
	h.deleteDuplicateService.Runner.CancelExecution()

	// Отдаем успешный ответ
	w.WriteHeader(http.StatusOK) // 200 OK
	w.Write([]byte(`{"status": "Сигнал остановки отправлен. Текущие таблицы дорабатывают и воркеры завершат работу."}`))
}

// GetLatestTasks TODO:  Берем последние незавершенные задачи updateAllProductList и loadAllNewProductList, не используется
func (h *TaskHandler) GetLatestTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.repo.GetLatestUnfinishedTasks()
	if err != nil {
		http.Error(w, "Ошибка базы данных", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
