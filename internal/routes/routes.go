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
func SetupRoutes(r *chi.Mux, taskHandler *handler.TaskHandler, localTaskHandler *handler.LocalTaskHandler, productListHandler *handler.ProductListHandler) {

	// Добавляем middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// ОДИН вызов Route для /api/v1
	r.Route(apiStr, func(r chi.Router) {

		// Группа для localTasks
		r.Get("/tasks/{id}", localTaskHandler.GetLocalTask)
		r.Post("/tasks", localTaskHandler.PutLocalTask)

		// Группа для allTasks
		r.Get("/allTasks/{id}", taskHandler.GetTask)
		r.Get("/allTasks/latest-unfinished", taskHandler.GetLatestTasks)
		r.Get("/allTasks/updateAllProductList", taskHandler.UpdateAllProductList)     // Запуск UpdateAllProductList
		r.Post("/allTasks/cancelAllProductList", taskHandler.CancelUpdateProductList) // Отмена UpdateAllProductList

		// Группа для динамических списков продуктов
		r.Get("/products/{listID}", productListHandler.GetList)
		r.Get("/product/{listID}/{itemID}", productListHandler.GetOne)
	})
}
