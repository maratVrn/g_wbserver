package parser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// Достаем свежие куки для вб с помощью chromedp

func GetWildberriesCookies() (string, error) {
	// Задаем настройки Chrome стандартными флагами строки
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.NoSandbox,
		// Передаем User-Agent прямо как флаг при старте браузера
		chromedp.Flag("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	// Сюда запишем куки в виде готовой строки
	var cookieString string

	err := chromedp.Run(ctx,
		// Открываем главную страницу WB
		chromedp.Navigate("https://wildberries.ru"),

		// Ждем появления футера (свидетельствует о загрузке страницы)
		chromedp.WaitVisible(`footer`, chromedp.ByQuery),

		// Даем 2 секунды скриптам WB на генерацию кук во внутреннем хранилище
		chromedp.Sleep(2*time.Second),

		// Забираем куки напрямую из JavaScript среды браузера
		chromedp.Evaluate(`document.cookie`, &cookieString),
	)
	if err != nil {
		return "", err
	}

	if cookieString == "" {
		return "", fmt.Errorf("браузер не смог сгенерировать cookie (возможно, капча)")
	}

	return cookieString, nil
}
