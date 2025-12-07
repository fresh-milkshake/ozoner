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
)

func main() {
	fmt.Fprintf(os.Stderr, "=== Тестирование парсера Ozon.ru ===\n")
	fmt.Fprintf(os.Stderr, "Задача: Спарсить 40 товаров по теме 'техника' и создать CSV файл\n\n")

	// Создаем клиент
	config := ozoner.DefaultConfig()
	config.UseBrowser = false
	config.RateLimit = 2.0
	config.Timeout = 45 * time.Second
	config.RetryAttempts = 2

	fmt.Fprintf(os.Stderr, "Создание клиента...\n")
	client, err := ozoner.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания клиента: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	fmt.Fprintf(os.Stderr, "✓ Клиент создан\n\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Собираем товары
	targetCount := 40
	allProducts := make([]ozoner.Product, 0, targetCount)

	fmt.Fprintf(os.Stderr, "Начинаем поиск товаров по запросу 'техника'...\n")
	fmt.Fprintf(os.Stderr, "Цель: собрать %d товаров\n\n", targetCount)

	// Пробуем получить товары с нескольких страниц
	page := 1
	maxPages := 10
	pageSize := 24

	for len(allProducts) < targetCount && page <= maxPages {
		fmt.Fprintf(os.Stderr, "[Страница %d] Загрузка...\n", page)

		// Use longer timeout for browser operations (they take more time)
		timeout := 45 * time.Second
		if config.UseBrowser {
			timeout = 120 * time.Second // Longer timeout for browser operations
		}
		searchCtx, searchCancel := context.WithTimeout(ctx, timeout)
		// Defer cancel to ensure context is cleaned up, but only after operation completes
		defer searchCancel()

		products, err := client.SearchProducts(searchCtx, "техника", &ozoner.SearchFilters{
			Page:     page,
			PageSize: pageSize,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "[Страница %d] Ошибка: %v\n", page, err)
			// Если первая страница не загрузилась, пробуем с браузером
			if page == 1 && !config.UseBrowser {
				fmt.Fprintf(os.Stderr, "Пробуем с браузером...\n")
				client.Close()
				config.UseBrowser = true
				client, err = ozoner.New(config)
				if err == nil {
					defer client.Close()
					// Use longer timeout for browser operations
					searchCtx2, searchCancel2 := context.WithTimeout(ctx, 120*time.Second)
					defer searchCancel2()

					products, err = client.SearchProducts(searchCtx2, "техника", &ozoner.SearchFilters{
						Page:     page,
						PageSize: pageSize,
					})
				}
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "[Страница %d] Пропущена из-за ошибки\n", page)
				page++
				if page > 3 {
					break // Прекращаем после 3 неудачных попыток
				}
				time.Sleep(3 * time.Second)
				continue
			}
		}

		if len(products) == 0 {
			fmt.Fprintf(os.Stderr, "[Страница %d] Товары не найдены\n", page)
			page++
			if page > 2 {
				break
			}
			continue
		}

		fmt.Fprintf(os.Stderr, "[Страница %d] Получено: %d товаров\n", page, len(products))

		// Добавляем товары
		for _, product := range products {
			if len(allProducts) >= targetCount {
				break
			}
			allProducts = append(allProducts, product)
		}

		if len(products) < pageSize {
			fmt.Fprintf(os.Stderr, "Достигнут конец результатов\n")
			break
		}

		page++
		time.Sleep(2 * time.Second)
	}

	if len(allProducts) == 0 {
		fmt.Fprintf(os.Stderr, "\n❌ Не удалось получить товары\n")
		fmt.Fprintf(os.Stderr, "Возможные причины:\n")
		fmt.Fprintf(os.Stderr, "1. Ozon.ru блокирует запросы (антибот-защита)\n")
		fmt.Fprintf(os.Stderr, "2. Изменилась структура API\n")
		fmt.Fprintf(os.Stderr, "3. Проблемы с сетью\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n✓ Всего собрано: %d товаров\n\n", len(allProducts))

	// Получаем детальную информацию (опционально, с таймаутом)
	fmt.Fprintf(os.Stderr, "Получение детальной информации...\n")
	detailedProducts := make([]ozoner.Product, 0, len(allProducts))

	for i, product := range allProducts {
		if product.ID == "" {
			detailedProducts = append(detailedProducts, product)
			continue
		}

		if (i+1)%10 == 0 {
			fmt.Fprintf(os.Stderr, "Обработано: %d/%d\n", i+1, len(allProducts))
		}

		// Use appropriate timeout for product details
		productCtx, productCancel := context.WithTimeout(ctx, 30*time.Second)
		defer productCancel()

		detailedProduct, err := client.GetProduct(productCtx, product.ID)

		if err != nil {
			// Используем данные из поиска
			detailedProducts = append(detailedProducts, product)
		} else {
			// Объединяем данные
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

		if (i+1)%5 == 0 {
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Fprintf(os.Stderr, "✓ Детальная информация получена\n\n")

	// Создаем CSV
	csvFilename := "technika_products.csv"
	fmt.Fprintf(os.Stderr, "Создание CSV файла: %s\n", csvFilename)

	file, err := os.Create(csvFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания CSV: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';' // Используем точку с запятой для лучшей совместимости
	defer writer.Flush()

	// Заголовки
	headers := []string{
		"ID",
		"Название",
		"Описание",
		"Цена",
		"Старая_цена",
		"Валюта",
		"Рейтинг",
		"Отзывов",
		"SKU",
		"Категория",
		"Бренд",
		"ID_продавца",
		"Название_продавца",
		"В_наличии",
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

		images := strings.Join(product.Images, " | ")
		if len(images) > 500 {
			images = images[:500] + "..."
		}

		attributes := formatAttributes(product.Attributes)
		if len(attributes) > 1000 {
			attributes = attributes[:1000] + "..."
		}

		description := strings.ReplaceAll(product.Description, "\n", " ")
		description = strings.ReplaceAll(description, "\r", " ")
		if len(description) > 500 {
			description = description[:500] + "..."
		}

		name := strings.ReplaceAll(product.Name, "\n", " ")
		name = strings.ReplaceAll(name, "\r", " ")

		record := []string{
			product.ID,
			name,
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
	}

	fmt.Fprintf(os.Stderr, "\n✓ CSV файл создан: %s\n", csvFilename)
	fmt.Fprintf(os.Stderr, "✓ Записано товаров: %d\n\n", len(detailedProducts))

	// Статистика
	fmt.Fprintf(os.Stderr, "=== СТАТИСТИКА ===\n")
	fmt.Fprintf(os.Stderr, "Всего товаров: %d\n", len(detailedProducts))

	stats := map[string]int{
		"С ценой":         0,
		"С рейтингом":     0,
		"С описанием":     0,
		"С изображениями": 0,
		"С атрибутами":    0,
	}

	for _, p := range detailedProducts {
		if p.Price > 0 {
			stats["С ценой"]++
		}
		if p.Rating > 0 {
			stats["С рейтингом"]++
		}
		if p.Description != "" {
			stats["С описанием"]++
		}
		if len(p.Images) > 0 {
			stats["С изображениями"]++
		}
		if len(p.Attributes) > 0 {
			stats["С атрибутами"]++
		}
	}

	for k, v := range stats {
		fmt.Fprintf(os.Stderr, "%s: %d\n", k, v)
	}

	fmt.Fprintf(os.Stderr, "\n=== ТЕСТИРОВАНИЕ ЗАВЕРШЕНО ===\n")
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
			valStr = strings.ReplaceAll(val, "\n", " ")
			valStr = strings.ReplaceAll(valStr, "\r", " ")
		case float64:
			valStr = fmt.Sprintf("%.2f", val)
		case int:
			valStr = strconv.Itoa(val)
		case bool:
			valStr = strconv.FormatBool(val)
		default:
			valStr = fmt.Sprintf("%v", val)
			valStr = strings.ReplaceAll(valStr, "\n", " ")
			valStr = strings.ReplaceAll(valStr, "\r", " ")
		}
		key := strings.ReplaceAll(k, "\n", " ")
		key = strings.ReplaceAll(key, "\r", " ")
		parts = append(parts, fmt.Sprintf("%s: %s", key, valStr))
	}
	return strings.Join(parts, " | ")
}
