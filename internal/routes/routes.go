package routes

import (
	"time"
	"wbserver/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

const (
	apiStr = "/api/v1"
)

// SetupRoutes настраивает все эндпоинты приложения
func SetupRoutes(r *chi.Mux, taskHandler *handler.TaskHandler, localTaskHandler *handler.LocalTaskHandler, productListHandler *handler.ProductListHandler,
	wbAnalyseHandler *handler.WBAnalyseHandler) {

	// Добавляем middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// ОДИН вызов Route для /api/v1
	r.Route(apiStr, func(r chi.Router) {

		// Группа для анализа данных
		// Получить список каталогов и предметов по запросу
		r.Get("/wb_analyse/findCatalogsBySubjectName", wbAnalyseHandler.FindCatalogsBySubjectName)

		// Возвращает список товаов с характеристиками по предмету ВБ (id)
		r.Get("/wb_analyse/findProductsBySubjectWbId", wbAnalyseHandler.FindProductsBySubjectWbId)

		// Новый эндпоинт для получения истории цен по ID товара
		r.Get("/wb_analyse/price-history", wbAnalyseHandler.GetProductPriceHistory)
		//  Отображение графиков на экране
		r.Get("/wb_analyse/price-chart", wbAnalyseHandler.ShowProductCharts)

		// Группа для localTasks
		r.Get("/tasks/{id}", localTaskHandler.GetLocalTask)
		r.Post("/tasks", localTaskHandler.PutLocalTask)

		// Группа для allTasks
		r.Get("/allTasks/wb_test", taskHandler.WbTest)
		r.Get("/allTasks/{id}", taskHandler.GetTask)
		r.Get("/allTasks/latest-unfinished", taskHandler.GetLatestTasks)
		r.Get("/allTasks/updateAllProductList", taskHandler.UpdateAllProductList)     // Запуск UpdateAllProductList
		r.Post("/allTasks/cancelAllProductList", taskHandler.CancelUpdateProductList) // Отмена UpdateAllProductList

		r.Get("/allTasks/deleteDuplicate", taskHandler.DeleteDuplicate)              // Запуск DeleteDuplicate
		r.Post("/allTasks/cancelDeleteDuplicate", taskHandler.CancelDeleteDuplicate) // Отмена CancelDeleteDuplicate

		// Группа для динамических списков продуктов
		r.Get("/products/{listID}", productListHandler.GetList)
		r.Get("/product/{listID}/{itemID}", productListHandler.GetOne)

		// Группа для работы с логами
		r.Post("/logger/clearAllLogFiles", handler.ClearLogsHandler) // Очистка всех логов
		r.Get("/logger/logs/{filename}", handler.GetLogFileHandler)  // Загрузка нужного kлог файлы

	})
}
