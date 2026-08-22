package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"wbserver/internal/database"
	"wbserver/internal/parser"
	"wbserver/internal/routes"
	"wbserver/internal/service"
	"wbserver/logger"

	"wbserver/internal/handler"
	"wbserver/internal/models"
	"wbserver/internal/repository"

	"github.com/go-chi/chi/v5"
)

const (
	httpPort = "8080"

	// Таймауты для HTTP-сервера
	readHeaderTimeout = 10 * time.Second
)

func main() {

	// Для оптимального энергопотребления чтобы не грузить полностью железо и не уводить в постоянное большое потребление энергии
	const maxWorkProcess = 4
	runtime.GOMAXPROCS(maxWorkProcess)
	numCPU := runtime.NumCPU()
	fmt.Println("Доступно ядер: ", numCPU, " ограничили до ", maxWorkProcess)
	// Инициализируем логгеры и получаем функции закрытия файлов
	closers := logger.InitLoggers()
	// Закрываем файлы логов при завершении работы программы
	defer func() {
		for _, closeFn := range closers {
			err := closeFn()
			if err != nil {
				// TODO: ошибку в файл системных ошибок
			}
		}
	}()

	// 1. Подключение к БД
	db := database.InitDB()

	// 2. Инициализация слоев
	repos := repository.NewRepositories(db)

	// Сервис для парсинга wb сначала получаем свежие куки
	wbCookie := "x_wbaas_token=1.1000.f11f303415174d6ea31fbefc6c62def3.MHwxMDkuMTA2LjEzNy4xNzR8TW96aWxsYS81LjAgKFdpbmRvd3MgTlQgMTAuMDsgV2luNjQ7IHg2NCkgQXBwbGVXZWJLaXQvNTM3LjM2IChLSFRNTCwgbGlrZSBHZWNrbykgQ2hyb21lLzEyMC4wLjAuMCBTYWZhcmkvNTM3LjM2fDE3ODAzMjA5MjR8cmV1c2FibGV8MnxleUpvWVhOb0lqb2lJbjA9fDB8M3wxNzgwMTkxMzI0fDE=.MEQCIGLBB+RsrA7+M86LJvplgoJOrPOcraGK2I7s5Tz/DzWXAiAi1xuyuPWvrz2fUKMZaLtbRhiFJ6/IxlGgSPr3L3534w==; _wbauid=9627114441780061727"
	//wbCookie, err := parser.GetWildberriesCookies()
	//if err != nil {
	//	fmt.Printf("getWildberriesCookies error: %v\n", err)
	//
	//}
	//fmt.Printf("cc: %v\n", wbCookie)

	parserService := parser.NewWBParserService(wbCookie)

	// Сервисы обновления данных
	updateRunner := service.NewTaskRunner()
	updateService := service.NewUpdateProductListService(repos.ProductList, repos.Tasks, parserService, updateRunner)

	// Сервис удаления дубликатов и сжатия данных до 1 г (запускать 1 раз в месяц)
	deleteDuplicateRunner := service.NewTaskRunner()
	deleteDuplicateService := service.NewDeleteDuplicateService(repos.ProductList, repos.Tasks, deleteDuplicateRunner)

	// Сервисы обновления данных
	loadRunner := service.NewTaskRunner()
	loadService := service.NewLoadProductListService(repos.ProductList, repos.Tasks, parserService, loadRunner)

	// Инициализируем планировщик задач
	//service.InitScheduler(updateService,deleteDuplicateService)

	// 2. Объединяем все фоновые сервисы в один слайс для запуска общих задач (завершение работы и т.п.)
	backgroundServices := []service.BackgroundService{
		updateService,
		deleteDuplicateService,
		loadService,
	}

	// Хендлеры
	taskHandler := handler.NewTaskHandler(repos.Tasks, repos.ProductList, updateService, deleteDuplicateService)
	productListHandler := handler.NewProductListHandler(repos.ProductList)
	wbAnalyseHandler := handler.NewWBAnalyseHandler(repos.WBAnalyse)

	// Инициализируем роутер Chi
	r := chi.NewRouter()
	// Создаем общее хранилище
	storage := models.NewTaskStorage()
	// Передаем его в хендлер
	localTaskHandler := &handler.LocalTaskHandler{
		Storage: storage,
	}
	routes.SetupRoutes(r, taskHandler, localTaskHandler, productListHandler, wbAnalyseHandler)

	// Запускаем HTTP-сервер
	server := &http.Server{
		Addr:              net.JoinHostPort("localhost", httpPort),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout, // Защита от Slowloris атак - тип DDoS-атаки
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("🚀 HTTP-сервер запущен на порту %s\n", httpPort)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("❌ Ошибка запуска сервера: %v\n", err)
		}
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("🛑 Получен сигнал остановки. Инициируем плавное завершение...")

	// выключаем HTTP-сервер, чтобы новые запросы физически не могли прийти
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	err := server.Shutdown(shutdownCtx)
	if err != nil {
		//TODO: ошибку в файл системных ошибок
	}

	// Посылаем сигналы отмены воркерам всем сервисов
	log.Println("Посылаем сигнал отмены во все фоновые сервисы...")
	for _, svc := range backgroundServices {
		svc.CancelExecution()
	}

	log.Println("Ожидаем завершения воркеров всех сервисов...")
	for i, svc := range backgroundServices {
		log.Printf("Ожидаем сервис №%d...", i+1)
		svc.GetWaitGroup().Wait() // Ждем, пока WaitGroup конкретного сервиса обнулится
	}

	log.Println("✅ Все воркеры успешно завершили работу. Сервер остановлен.")

}
