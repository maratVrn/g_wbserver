package analytics

// СОбираем инфо о товарах продавца - через DOM элементы
// Скролимся по странице вниз пока не упремся в футер
import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

func FindSellerProducts(sellerId string) error {

	url := "https://www.wildberries.ru/seller/" + sellerId

	// 2. Создаем/очищаем файл результатов и пишем заголовки (выполняется ОДИН раз при старте)
	outputFile := "seller\\temp.txt"
	file, err := os.Create(outputFile)
	if err == nil {
		writer := csv.NewWriter(file)
		writer.Comma = ';'
		// Заголовки колонок
		_ = writer.Write([]string{"ID", "Цена Кошелек", "Цена СПП", "Рейтинг", "Отзывы"})
		writer.Flush()
		file.Close()
	}

	// 1. Настройка браузера для обхода блокировок
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// Флаг, который разворачивает окно на максимум при старте
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("headless", false), // ОБЯЗАТЕЛЬНО false для отладки, иначе WB сразу выдаст капчу
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// Используем актуальный User-Agent (минимум Chrome 125+)
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	// Создаем ОДИН стабильный контекст браузера на все время работы программы
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	log.Println("Открываем страницу продавца Wildberries...")

	// Выполняем переход на страницу продавца напрямую через стабильный контекст
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(url),
		// Ждем появления первой карточки товара в DOM
		chromedp.WaitVisible(`.product-card`, chromedp.ByQuery),
	)
	if err != nil {
		log.Fatalf("Ошибка загрузки страницы: %v", err)
	}

	productsMap := make(map[string]ProductDOM)

	log.Println("Запуск плавного скроллинга и глубокого парсинга карточек...")

	bottomHits := 0
	maxBottomHits := 6
	var lastScrollHeight int

	for {
		var isBottom bool
		var currentScrollHeight int
		var stepProducts []ProductDOM

		err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			err := chromedp.Evaluate(`
				var footer = document.querySelector('.footer-lands') || document.querySelector('footer');
				if (footer) {
					footer.scrollIntoView({ behavior: 'smooth', block: 'end' });
				} else {
					window.scrollBy(0, 600);
				}
			`, nil).Do(ctx)
			if err != nil {
				return err
			}

			time.Sleep(500 * time.Millisecond)

			// УМНЫЙ DOM ПАРСЕР: Собираем ID, цены, рейтинг и оценки
			err = chromedp.Evaluate(`
				Array.from(document.querySelectorAll('.product-card')).map(card => {
					let id = card.getAttribute('data-nm-id');
					if (!id) return null;

					// Находим элементы цен
					let priceEl = card.querySelector('ins');
					let oldPriceEl = card.querySelector('del');

					// Находим элементы рейтинга и отзывов по data-testid
					let ratingEl = card.querySelector('[data-testid="catalogCardRating"]');
					let commentEl = card.querySelector('[data-testid="catalogCardCommentCount"]');

					// Очищаем цены от '₽' и пробелов
					let price = priceEl ? priceEl.innerText.replace(/[^\d]/g, '') : '';
					let oldPrice = oldPriceEl ? oldPriceEl.innerText.replace(/[^\d]/g, '') : '';

					// Очищаем рейтинг: меняем запятую на точку и убираем лишнее
					let rating = ratingEl ? ratingEl.innerText.replace(',', '.').replace(/[^\d.]/g, '') : '';

					// Очищаем отзывы: убираем слово "оценки", оставляем только число
					let commentCount = commentEl ? commentEl.innerText.replace(/[^\d]/g, '') : '';

					return { 
						id: id, 
						price: price, 
						oldPrice: oldPrice,
						rating: rating,
						commentCount: commentCount
					};
				}).filter(item => item !== null)
			`, &stepProducts).Do(ctx)
			if err != nil {
				return err
			}

			if err := chromedp.Evaluate(`document.documentElement.scrollHeight`, &currentScrollHeight).Do(ctx); err != nil {
				return err
			}

			return chromedp.Evaluate(`
				(window.innerHeight + window.scrollY) >= (document.documentElement.scrollHeight - 120)
			`, &isBottom).Do(ctx)
		}))

		if err != nil {
			log.Printf("Предупреждение при обработке шага: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, p := range stepProducts {
			productsMap[p.ID] = p
		}
		log.Printf("На шаге считано из DOM: %d карточек. Всего уникальных товаров в базе: %d", len(stepProducts), len(productsMap))

		if isBottom {
			if currentScrollHeight > lastScrollHeight {
				bottomHits = 0
				lastScrollHeight = currentScrollHeight
				log.Println("Страница увеличилась. Ожидаем отрисовку новых товаров...")
			} else {
				bottomHits++
				if bottomHits >= maxBottomHits {
					log.Println("Сбор данных полностью завершен.")
					break
				}

				_ = chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
					_ = chromedp.Evaluate(`window.scrollBy(0, -300);`, nil).Do(ctx)
					time.Sleep(300 * time.Millisecond)
					_ = chromedp.Evaluate(`window.scrollBy(0, 300);`, nil).Do(ctx)
					return nil
				}))
			}
			time.Sleep(1500 * time.Millisecond)
		} else {
			bottomHits = 0
			lastScrollHeight = currentScrollHeight
			time.Sleep(1200 * time.Millisecond)
		}
	}

	// ИТОГОВЫЙ ВЫВОД В КОНСОЛЬ
	fmt.Println("\n==========================================================================================")
	fmt.Printf(" СБОР ПОЛНОСТЬЮ ЗАВЕРШЕН. Всего уникальных товаров сохранено: %d\n", len(productsMap))
	fmt.Println("==========================================================================================")

	counter := 0
	log.Println("Пример извлеченных данных (первые 10 товаров):")
	for _, p := range productsMap {
		fmt.Printf("ID: %s | Цена: %s | Старая: %s | Рейтинг: %s | Оценок: %s\n",
			p.ID, p.Price, p.OldPrice, p.Rating, p.CommentCount)
		counter++
		if counter >= 10 {
			break
		}
	}

	fmt.Println("==========================================================================")
	for _, info := range productsMap {
		row := []string{info.ID, info.Price, info.Price, info.Rating, info.CommentCount}
		saveToCSV(outputFile, row)

	}

	finalFile := "seller\\allResults.txt" // Исправили .cvs на .csv

	err = os.Rename(outputFile, finalFile)
	if err != nil {
		log.Printf("Не удалось переименовать файл: %v", err)
	} else {
		log.Printf("Результаты успешно сохранены в: %s", finalFile)
	}
	return nil
}

// Product DOM — структура для хранения полной информации из верстки страницы
type ProductDOM struct {
	ID           string `json:"id"`
	Price        string `json:"price"`        // Актуальная цена (из тега ins)
	OldPrice     string `json:"oldPrice"`     // Старая цена (из тега del)
	Rating       string `json:"rating"`       // Рейтинг (например, 4.9)
	CommentCount string `json:"commentCount"` // Количество оценок (например, 24)
}
