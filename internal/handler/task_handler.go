package handler

import (
	"encoding/json"
	"net/http"
	"wbserver/internal/repository"
	"wbserver/internal/service"

	"github.com/go-chi/chi/v5"
)

type TaskHandler struct {
	repo            *repository.TaskRepository
	productListRepo *repository.ProductListRepository
	updateService   *service.UpdateProductListService
}

func NewTaskHandler(repo *repository.TaskRepository, productListRepo *repository.ProductListRepository, updateService *service.UpdateProductListService) *TaskHandler {
	return &TaskHandler{repo: repo, productListRepo: productListRepo, updateService: updateService}
}

// TODO:  Получить задачу по ID вроде не испольщуется
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

// Остановить UpdateAllProductList
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

// TODO: передать логи в отдельный файл задач

// Глобальная задача - обновление всех ИД продуктов
func (h *TaskHandler) UpdateAllProductList(w http.ResponseWriter, r *http.Request) {

	//w.Header().Set("Content-Type", "application/json")
	//// 1. Мгновенно проверяем, не занят ли сервис фоновой работой
	//if h.updateService.Runner.IsBusy() {
	//	w.WriteHeader(http.StatusConflict) // Код ответа 409 (Конфликт состояния)
	//	w.Write([]byte(`{"error": "Процесс обновления уже запущен и выполняется в данный момент"}`))
	//	return
	//}
	////1.  Получаем список таблиц на обновление данных (либо в текущей задаче либо создаем новую)
	//task, unfinishedTables, err := h.repo.GetLatestUnfinishedUpdateTask()
	//if err != nil {
	//	errMsg := fmt.Sprintf("Ошибка базы данных: %v", err)
	//	http.Error(w, errMsg, http.StatusInternalServerError)
	//	return
	//}
	//// 2. Запускаем воркер-пул в фоновом потоке Передаем context.Background(), у которого НЕТ никаких скрытых таймаутов сервера
	//go func() {
	//	err := h.updateService.UpdateAllProductList(context.Background(), task, unfinishedTables)
	//	if err != nil {
	//		log.Printf("[Background Task UpdateAllProductList] Процесс завершился с ошибкой: %v", err)
	//		// Здесь можно обновить статус задачи в БД через taskRepo на "Ошибка"
	//		return
	//	}
	//	log.Println("[Background Task] Процесс успешно завершен!")
	//
	//}()
	//
	//json.NewEncoder(w).Encode("процесс обновления успешно запущен в фоне")

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

// TODO:  Берем последние незавершенные задачи updateAllProductList и loadAllNewProductList, не используется
func (h *TaskHandler) GetLatestTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.repo.GetLatestUnfinishedTasks()
	if err != nil {
		http.Error(w, "Ошибка базы данных", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
