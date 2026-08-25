package analytics

// Функция фильтрации исходных данных получаемых при методе FindProductsBySubjectWbId->SaveResultsToCSV

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// FilterQueryParams описывает структуру запроса с параметрами фильтрации
type FilterQueryParams struct {
	MinMedianPrice         int
	MaxMedianPrice         int
	MinMedianMonthlySales  int
	MaxMedianMonthlySales  int
	MinMatchingMonthsCount int
}

// FilteredData фильтруем данные собранные с БД по нужным условиям FilterQueryParams
func FilteredData(fileName string) {
	// Пример настройки параметров вашего запроса
	query := FilterQueryParams{
		MinMedianPrice:         100,
		MaxMedianPrice:         600,
		MinMedianMonthlySales:  20,
		MaxMedianMonthlySales:  5000,
		MinMatchingMonthsCount: 5,
	}

	inputPath := "analytics\\" + fileName
	outputPath := "analytics\\Filtered_" + fileName

	fmt.Println("Запуск фильтрации данных...")
	err := FilterAndSaveCSV(inputPath, outputPath, query)
	if err != nil {
		fmt.Printf("[ОШИБКА]: %v\n", err)
		return
	}

	fmt.Printf("Фильтрация успешно завершена! Данные сохранены в файл: %s\n", outputPath)
}

// FindNewFilteredData Сравниваем 2 отфильтрованных файл, чтобы добавить новые товары в исследование
func FindNewFilteredData() {
	fileNameAll := "analytics\\Filtered_Проволока_6006_all.csv"
	fileNameOld := "analytics\\Filtered_Проволока_6006_1.csv"

	// 1. Открываем файл, в котором находятся ID для исключения
	file1, err := os.Open(fileNameOld)
	if err != nil {
		fmt.Printf("Ошибка открытия file_1: %v\n", err)
		return
	}
	defer file1.Close()

	// Настраиваем CSV reader для первого файла (разделитель ';')
	reader1 := csv.NewReader(file1)
	reader1.Comma = ';'

	// Читаем заголовок первого файла, чтобы пропустить его
	_, err = reader1.Read()
	if err != nil {
		fmt.Printf("Ошибка чтения заголовка file_1: %v\n", err)
		return
	}

	// Собираем все ID из первого файла в map для быстрого поиска
	excludedIDs := make(map[string]bool)
	for {
		record, err := reader1.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Ошибка чтения строки из file_1: %v\n", err)
			return
		}
		// ID находится в первом столбце (индекс 0)
		excludedIDs[record[0]] = true
	}

	// 2. Открываем файл со всеми данными
	fileAll, err := os.Open(fileNameAll)
	if err != nil {
		fmt.Printf("Ошибка открытия file_all: %v\n", err)
		return
	}
	defer fileAll.Close()

	readerAll := csv.NewReader(fileAll)
	readerAll.Comma = ';'

	// Читаем заголовок всеобщего файла
	header, err := readerAll.Read()
	if err != nil {
		fmt.Printf("Ошибка чтения заголовка file_all: %v\n", err)
		return
	}

	// 3. Создаем новый файл для сохранения результатов
	fileOut, err := os.Create("analytics\\add_data.csv")
	if err != nil {
		fmt.Printf("Ошибка создания файла add_data: %v\n", err)
		return
	}
	defer fileOut.Close()

	// Настраиваем CSV writer для записи (разделитель ';')
	writer := csv.NewWriter(fileOut)
	writer.Comma = ';'
	defer writer.Flush()

	// Записываем заголовок в новый файл
	if err := writer.Write(header); err != nil {
		fmt.Printf("Ошибка записи заголовка: %v\n", err)
		return
	}

	// 4. Фильтруем строки и записываем уникальные данные
	count := 0
	for {
		record, err := readerAll.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Ошибка чтения строки из file_all: %v\n", err)
			return
		}

		id := record[0]
		// Если ID отсутствует в map исключений — записываем строку
		if !excludedIDs[id] {
			if err := writer.Write(record); err != nil {
				fmt.Printf("Ошибка записи строки: %v\n", err)
				return
			}
			count++
		}
	}

	fmt.Printf("Готово! В файл 'add_data.csv' успешно добавлено строк: %d\n", count)
}

// parseNumeric обрабатывает строки, удаляя пробелы, переводы строк и округляя float-значения (например "37.0" -> 37)
func parseNumeric(s string) (int, error) {
	// Очищаем от пробелов, \r и \n
	s = strings.TrimSpace(s)

	// Если число записано как float (например, "37.0"), парсим в float и приводим к int
	if strings.Contains(s, ".") {
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		return int(val), nil
	}

	// Обычный парсинг целого числа
	return strconv.Atoi(s)
}

// FilterAndSaveCSV считывает данные из inputFile, фильтрует по заданным параметрам и сохраняет в outputFile
func FilterAndSaveCSV(inputFile, outputFile string, query FilterQueryParams) error {
	srcFile, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("ошибка открытия исходного файла: %w", err)
	}
	defer srcFile.Close()

	reader := csv.NewReader(srcFile)
	reader.Comma = ';'
	// Автоматически обрезать пробелы в начале полей
	reader.TrimLeadingSpace = true

	var filteredRecords [][]string
	isFirstRow := true

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break // Конец файла
		}
		if err != nil {
			return fmt.Errorf("ошибка чтения строки: %w", err)
		}

		if len(row) < 4 {
			continue
		}

		// ПРОВЕРКА НА НАЛИЧИЕ ЗАГОЛОВКОВ:
		// Если первая строка текстовая (например, содержит "ID"), сохраняем её как заголовок и идем дальше
		if isFirstRow {
			isFirstRow = false
			if _, errCheck := strconv.Atoi(strings.TrimSpace(row[0])); errCheck != nil {
				// Это строка заголовков, просто пропускаем её фильтрацию, но добавим в результат позже
				filteredRecords = append(filteredRecords, row)
				continue
			}
		}

		// Безопасный парсинг с очисткой от спецсимволов \r и поддержкой float
		medianPrice, err1 := parseNumeric(row[1])
		medianMonthlySales, err2 := parseNumeric(row[2])
		matchingMonthsCount, err3 := parseNumeric(row[3])

		// Если хоть одно число не распарсилось, выводим ошибку в консоль для диагностики и пропускаем строку
		if err1 != nil || err2 != nil || err3 != nil {
			fmt.Printf("[Предупреждение] Пропущена некорректная строка %v: err1=%v, err2=%v, err3=%v\n", row, err1, err2, err3)
			continue
		}

		// Жесткие условия фильтрации
		if medianPrice < query.MinMedianPrice || medianPrice > query.MaxMedianPrice {
			continue
		}
		if medianMonthlySales < query.MinMedianMonthlySales || medianMonthlySales > query.MaxMedianMonthlySales {
			continue
		}
		if matchingMonthsCount < query.MinMatchingMonthsCount {
			continue
		}

		// Строка полностью валидна и прошла фильтр
		filteredRecords = append(filteredRecords, row)
	}

	// Запись результатов
	dstFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("ошибка создания результирующего файла: %w", err)
	}
	defer dstFile.Close()

	writer := csv.NewWriter(dstFile)
	writer.Comma = ';'
	// Windows-friendly перенос строк для выходного файла
	writer.UseCRLF = true
	defer writer.Flush()

	if err := writer.WriteAll(filteredRecords); err != nil {
		return fmt.Errorf("ошибка записи данных: %w", err)
	}

	return nil
}
