package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ozoner "ozoner/pkg/ozoner"

	"github.com/charmbracelet/log"
)

func main() {
	// Настройка логирования - отключаем для чистого вывода
	log.SetLevel(log.ErrorLevel)

	// Создаем файл для логирования
	logFile, _ := os.Create("test_log.txt")
	defer logFile.Close()

	fmt.Println("=== Тестирование парсера Ozon.ru ===")
	fmt.Println()
	fmt.Println("Задача: Спарсить 40 товаров по теме 'техника' и создать CSV файл")
	fmt.Println()
	fmt.Fprintf(logFile, "Начало теста\n")
	logFile.Sync()
	os.Stdout.Sync()

	// Создаем клиент
	config := ozoner.DefaultConfig()
	config.UseBrowser = false // Сначала пробуем без браузера
	config.RateLimit = 3.0
	config.Timeout = 60 * time.Second
	config.RetryAttempts = 3

	log.Info("Инициализация клиента",
		"use_browser", config.UseBrowser,
		"rate_limit", config.RateLimit)

	client, err := ozoner.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания клиента: %v\n", err)
		fmt.Fprintf(logFile, "Ошибка создания клиента: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("✓ Клиент создан успешно")
	fmt.Println()
	fmt.Fprintf(logFile, "Клиент создан\n")
	logFile.Sync()
	os.Stdout.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Собираем 40 товаров
	targetCount := 40
	allProducts := make([]ozoner.Product, 0, targetCount)

	fmt.Printf("Начинаем поиск товаров по запросу 'техника'...\n")
	fmt.Printf("Цель: собрать %d товаров\n", targetCount)
	fmt.Println()
	os.Stdout.Sync()

	// Пробуем получить товары с нескольких страниц
	page := 1
	maxPages := 5  // Максимум 5 страниц
	pageSize := 20 // По 20 товаров на странице

	for len(allProducts) < targetCount && page <= maxPages {
		fmt.Printf("Загрузка страницы %d...\n", page)
		os.Stdout.Sync()

		searchCtx, searchCancel := context.WithTimeout(ctx, 30*time.Second)
		fmt.Printf("Выполняем запрос к API...\n")
		os.Stdout.Sync()
		products, err := client.SearchProducts(searchCtx, "техника", &ozoner.SearchFilters{
			Page:     page,
			PageSize: pageSize,
		})
		searchCancel()

		if err != nil {
			fmt.Printf("Ошибка API: %v\n", err)
			os.Stdout.Sync()
		}

		if err != nil {
			log.Warn("Ошибка при загрузке страницы", "page", page, "err", err)
			// Если первая страница не загрузилась, пробуем с браузером
			if page == 1 && !config.UseBrowser {
				fmt.Println("\n⚠ API не отвечает, пробуем с браузером...")
				client.Close()
				config.UseBrowser = true
				client, err = ozoner.New(config)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Ошибка создания клиента с браузером: %v\n", err)
					os.Exit(1)
				}
				// Повторяем попытку
				products, err = client.SearchProducts(ctx, "техника", &ozoner.SearchFilters{
					Page:     page,
					PageSize: pageSize,
				})
			}

			if err != nil {
				fmt.Printf("⚠ Ошибка на странице %d: %v\n", page, err)
				page++
				continue
			}
		}

		if len(products) == 0 {
			fmt.Printf("⚠ На странице %d товаров не найдено\n", page)
			page++
			if page > 2 {
				break // Если 2 страницы подряд пустые, прекращаем
			}
			continue
		}

		fmt.Printf("✓ Получено товаров на странице %d: %d\n", page, len(products))
		os.Stdout.Sync()

		// Добавляем товары
		for _, product := range products {
			if len(allProducts) >= targetCount {
				break
			}
			allProducts = append(allProducts, product)
		}

		// Если получили меньше чем pageSize, значит это последняя страница
		if len(products) < pageSize {
			fmt.Println("Достигнут конец результатов")
			break
		}

		page++
		time.Sleep(2 * time.Second) // Небольшая задержка между страницами
	}

	if len(allProducts) == 0 {
		fmt.Fprintf(os.Stderr, "\n❌ Не удалось получить товары. Возможные причины:\n")
		fmt.Fprintf(os.Stderr, "1. Ozon.ru блокирует запросы (антибот-защита)\n")
		fmt.Fprintf(os.Stderr, "2. Изменилась структура API\n")
		fmt.Fprintf(os.Stderr, "3. Проблемы с сетью\n")
		os.Exit(1)
	}

	fmt.Printf("\n✓ Всего собрано товаров: %d\n\n", len(allProducts))

	// Получаем детальную информацию для каждого товара
	fmt.Println("Получение детальной информации о товарах...")
	detailedProducts := make([]ozoner.Product, 0, len(allProducts))

	for i, product := range allProducts {
		if product.ID == "" {
			fmt.Printf("⚠ Товар #%d: пропущен (нет ID)\n", i+1)
			detailedProducts = append(detailedProducts, product)
			continue
		}

		fmt.Printf("Загрузка деталей товара %d/%d: %s...\n", i+1, len(allProducts), truncate(product.Name, 50))
		os.Stdout.Sync()

		// Получаем детальную информацию
		productCtx, productCancel := context.WithTimeout(ctx, 20*time.Second)
		detailedProduct, err := client.GetProduct(productCtx, product.ID)
		productCancel()
		if err != nil {
			log.Warn("Не удалось получить детали товара", "id", product.ID, "err", err)
			// Используем данные из поиска
			detailedProducts = append(detailedProducts, product)
		} else {
			// Объединяем данные: детали имеют приоритет, но сохраняем то, что было в поиске
			if detailedProduct.Name == "" {
				detailedProduct.Name = product.Name
			}
			if detailedProduct.Price == 0 {
				detailedProduct.Price = product.Price
			}
			if detailedProduct.URL == "" {
				detailedProduct.URL = product.URL
			}
			if len(detailedProduct.Images) == 0 && len(product.Images) > 0 {
				detailedProduct.Images = product.Images
			}
			detailedProducts = append(detailedProducts, *detailedProduct)
		}

		// Небольшая задержка между запросами
		if (i+1)%5 == 0 {
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Printf("✓ Детальная информация получена для %d товаров\n\n", len(detailedProducts))

	// Создаем CSV файл
	csvFilename := "technika_products.csv"
	fmt.Printf("Создание CSV файла: %s\n", csvFilename)

	file, err := os.Create(csvFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания CSV файла: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовки CSV
	headers := []string{
		"ID",
		"Название",
		"Описание",
		"Цена",
		"Старая цена",
		"Валюта",
		"Рейтинг",
		"Количество отзывов",
		"SKU",
		"Категория",
		"Бренд",
		"ID продавца",
		"Название продавца",
		"В наличии",
		"URL",
		"Изображения",
		"Атрибуты",
	}

	if err := writer.Write(headers); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка записи заголовков: %v\n", err)
		os.Exit(1)
	}

	// Записываем данные
	for i, product := range detailedProducts {
		oldPrice := ""
		if product.OldPrice != nil {
			oldPrice = fmt.Sprintf("%.2f", *product.OldPrice)
		}

		images := strings.Join(product.Images, "; ")
		if len(images) > 500 {
			images = images[:500] + "..."
		}

		// Форматируем атрибуты
		attributes := formatAttributes(product.Attributes)
		if len(attributes) > 1000 {
			attributes = attributes[:1000] + "..."
		}

		description := product.Description
		if len(description) > 500 {
			description = description[:500] + "..."
		}

		record := []string{
			product.ID,
			product.Name,
			description,
			fmt.Sprintf("%.2f", product.Price),
			oldPrice,
			product.Currency,
			fmt.Sprintf("%.2f", product.Rating),
			strconv.Itoa(product.ReviewsCount),
			product.SKU,
			product.Category,
			product.Brand,
			product.SellerID,
			product.SellerName,
			strconv.FormatBool(product.InStock),
			product.URL,
			images,
			attributes,
		}

		if err := writer.Write(record); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка записи строки %d: %v\n", i+1, err)
			continue
		}

		if (i+1)%10 == 0 {
			fmt.Printf("Записано товаров в CSV: %d/%d\n", i+1, len(detailedProducts))
		}
	}

	fmt.Printf("\n✓ CSV файл создан успешно: %s\n", csvFilename)
	fmt.Printf("✓ Всего записано товаров: %d\n\n", len(detailedProducts))

	// Выводим статистику
	fmt.Println("=== СТАТИСТИКА ===")
	fmt.Printf("Всего товаров: %d\n", len(detailedProducts))

	withPrice := 0
	withRating := 0
	withDescription := 0
	withImages := 0
	withAttributes := 0

	for _, p := range detailedProducts {
		if p.Price > 0 {
			withPrice++
		}
		if p.Rating > 0 {
			withRating++
		}
		if p.Description != "" {
			withDescription++
		}
		if len(p.Images) > 0 {
			withImages++
		}
		if len(p.Attributes) > 0 {
			withAttributes++
		}
	}

	fmt.Printf("С ценой: %d\n", withPrice)
	fmt.Printf("С рейтингом: %d\n", withRating)
	fmt.Printf("С описанием: %d\n", withDescription)
	fmt.Printf("С изображениями: %d\n", withImages)
	fmt.Printf("С атрибутами: %d\n", withAttributes)

	fmt.Println("\n=== ТЕСТИРОВАНИЕ ЗАВЕРШЕНО УСПЕШНО ===")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatAttributes(attrs map[string]interface{}) string {
	if len(attrs) == 0 {
		return ""
	}

	var parts []string
	for k, v := range attrs {
		var valStr string
		switch val := v.(type) {
		case string:
			valStr = val
		case float64:
			valStr = fmt.Sprintf("%.2f", val)
		case int:
			valStr = strconv.Itoa(val)
		case bool:
			valStr = strconv.FormatBool(val)
		default:
			valStr = fmt.Sprintf("%v", val)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, valStr))
	}
	return strings.Join(parts, "; ")
}
