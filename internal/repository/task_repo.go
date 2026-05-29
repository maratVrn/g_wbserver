package repository

import (
	"encoding/json"
	"fmt"
	"time"
	"wbserver/internal/models"

	"gorm.io/gorm"
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) FindByID(id string) (*models.Task, error) {
	var task models.Task
	// Поиск по ID
	if err := r.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// Создаем новую задачу на обновление данных updateAllProductList
func (r *TaskRepository) setNewUpdateTask() (*models.Task, []string, error) {

	var tableNames []string

	err := r.db.Raw(`
	SELECT relname
	FROM pg_class
	JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
	WHERE pg_namespace.nspname = 'public'
	 AND relname LIKE 'productList%'
	 AND relkind = 'r'
	 AND reltuples > 0
	`).Scan(&tableNames).Error

	if err != nil {
		return nil, nil, fmt.Errorf("ошибка получения списка таблиц: %v", err)
	}

	if len(tableNames) == 0 {
		return nil, nil, fmt.Errorf("таблицы productList% не найдены в БД")
	}

	// Формируем слайс структур для JSON и слайс имен для возврата
	var productTasks []models.UpdateTask
	for _, tn := range tableNames {
		productTasks = append(productTasks, models.UpdateTask{
			TableName:    tn,
			TableTaskEnd: false,
		})
	}

	// Сериализуем данные в JSON
	jsonData, err := json.Marshal(productTasks)
	if err != nil {

		return nil, nil, fmt.Errorf("ошибка маршалинга JSON: %v", err)
	}

	// Инициализируем и сохраняем новую задачу в БД
	newTask := models.Task{
		TaskName:      "updateAllProductList",
		IsEnd:         false,
		TaskData:      jsonData, // Поле типа datatypes.JSON примет []byte
		StartDateTime: time.Now().Format("02.01.2006"),
	}
	if err := r.db.Create(&newTask).Error; err != nil {
		return nil, nil, fmt.Errorf("ошибка создания новой задачи: %v", err)
	}

	return &newTask, tableNames, nil
}

// Возвращает последнюю незавершенную задачу updateAllProductList если она есть
func (r *TaskRepository) GetLatestUnfinishedUpdateTask() (*models.Task, []string, error) {
	var task models.Task          // Сама задача
	var unfinishedTables []string // Список таблиц на обновление
	taskType := "updateAllProductList"

	// Ищем последнюю незавершенную задачу конкретного типа
	err := r.db.Where("\"taskName\" = ? AND \"isEnd\" = ?", taskType, false).
		Order("id DESC").
		First(&task).Error

	if err != nil {
		// Если задачи нет создаем новую
		if err == gorm.ErrRecordNotFound {
			var newTask *models.Task
			newTask, unfinishedTables, err = r.setNewUpdateTask()
			if err != nil {
				return nil, unfinishedTables, fmt.Errorf("Ошибка создания новой задачи: %v", err)
			}
			return newTask, unfinishedTables, nil
		}
		return nil, unfinishedTables, err
	}

	// Достаем список НЕ обработанных таблиц
	var updateTasks []models.UpdateTask
	err2 := json.Unmarshal(task.TaskData, &updateTasks)
	if err2 != nil {
		return nil, unfinishedTables, fmt.Errorf("ошибка парсинга TaskData: %v", err2)
	}

	for _, pt := range updateTasks {
		if !pt.TableTaskEnd {
			unfinishedTables = append(unfinishedTables, pt.TableName)
		}
	}

	return &task, unfinishedTables, nil
}

// Cохраняем прогресс updateAllProductList в БД
func (r *TaskRepository) SaveUpdateProductListProgress(taskID uint, data []models.UpdateTask) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Ошибка маршалинга при сохранении прогресса: %v\n", err)
		return err
	}

	// Обновляем только поле taskData для конкретного ID
	err = r.db.Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("taskData", jsonData).Error

	if err != nil {
		fmt.Printf("Ошибка записи прогресса в БД: %v\n", err)
		return err
	}

	return nil
}

func (r *TaskRepository) SaveUpdateProductListIsEnd(taskID uint) error {
	err := r.db.Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("isEnd", true).Error
	if err != nil {
		fmt.Printf("Ошибка Сохранения стасу задачи UpdateProductListIsEnd БД: %v\n", err)
	}
	return nil
}
func (r *TaskRepository) GetLatestUnfinishedTasks() (map[string]models.Task, error) {
	results := make(map[string]models.Task)
	taskTypes := []string{"updateAllProductList", "loadAllNewProductList"}

	for _, tType := range taskTypes {
		var task models.Task
		// Ищем задачи, где taskName совпадает, а isEnd = false
		// Сортируем по ID убыванию, чтобы взять самую новую (последнюю)
		err := r.db.Where("\"taskName\" = ? AND \"isEnd\" = ?", tType, false).
			Order("id DESC").
			First(&task).Error

		if err == nil {
			results[tType] = task
		} else if err != gorm.ErrRecordNotFound {
			return nil, err // Возвращаем ошибку, если это не "не найдено"
		}
	}

	return results, nil
}
