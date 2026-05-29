package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"wbserver/internal/repository"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type ProductListHandler struct {
	repo *repository.ProductListRepository
}

func NewProductListHandler(repo *repository.ProductListRepository) *ProductListHandler {
	return &ProductListHandler{repo: repo}
}

// GetList возвращает все товары из динамической таблицы productListXXXXXX
func (h *ProductListHandler) GetList(w http.ResponseWriter, r *http.Request) {
	// 1. Получаем listID из URL (например, из /api/v1/products/131939)
	listID := chi.URLParam(r, "listID")

	// 2. ВАЖНО: Валидация. Так как мы подставляем listID напрямую в имя таблицы,
	// нужно убедиться, что там только цифры (защита от SQL Injection).
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(listID) {
		http.Error(w, "Некорректный ID списка", http.StatusBadRequest)
		return
	}

	// 3. Вызов репозитория
	products, err := h.repo.GetItemsFromList(listID)
	if err != nil {
		// Если таблицы не существует или ошибка запроса
		http.Error(w, "Ошибка при получении данных: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Отправка ответа
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, "Ошибка формирования JSON", http.StatusInternalServerError)
	}
}

func (h *ProductListHandler) GetOne(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "listID")
	itemIDStr := chi.URLParam(r, "itemID")

	// Конвертируем строку "3660035" в число 3660035
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "ID товара должен быть числом", http.StatusBadRequest)
		return
	}

	item, err := h.repo.GetItemByID(listID, itemID)
	fmt.Println(item)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Товар не найден", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(item)
}
