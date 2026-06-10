package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"wbserver/logger"

	"github.com/go-chi/chi/v5"
)

func ClearLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Итерируемся по ключам нашей общей карты
	for filename := range logger.AllowedLogFiles {
		fullPath := filepath.Join(logger.LogDir, filename)

		err := os.Truncate(fullPath, 0)
		if err != nil {
			//Error.Printf("Не удалось очистить файл %s: %v\n", fullPath, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Все файлы логов успешно очищены"))
}

func GetLogFileHandler(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// проверка: сверяемся с единым списком логов
	if !logger.AllowedLogFiles[filename] {
		http.Error(w, "File not found or access denied", http.StatusNotFound)
		return
	}

	fullPath := filepath.Join(logger.LogDir, filename)
	http.ServeFile(w, r, fullPath)
}
