package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	ozoner "ozoner/pkg/ozoner"
)

func main() {
	// Настройка логирования
	log.SetLevel(log.DebugLevel) // Включаем подробное логирование
	
	fmt.Println("=== Парсинг товаров с Ozon.ru ===\n")
	fmt.Println("Тема: техника\n")

	// Создаем клиент с browser support
	config := ozoner.DefaultConfig()
	config.UseBrowser = true  // Включаем browser client для обхода антибот-защиты
	config.RateLimit = 2.0    // Низкий rate limit для избежания блокировок
	config.Timeout = 60 * time.Second

	log.Info("Инициализация клиента с browser support", 
		"use_browser", config.UseBrowser,
		"rate_limit", config.RateLimit,
		"timeout", config.Timeout)
	
	client, err := ozoner.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания клиента: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	log.Info("Клиент создан успешно")
	fmt.Println("✓ Клиент создан успешно\n")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Парсим товары по запросу "техника"
	log.Info("Начинаем поиск товаров", "query", "техника", "page_size", 4)
	fmt.Println("Поиск товаров по запросу 'техника'...")
	
	products, err := client.SearchProducts(ctx, "техника", &ozoner.SearchFilters{
		Page:     1,
		PageSize: 4, // Получаем 4 товара
	})

	if err != nil {
		log.Error("Ошибка поиска", "err", err)
		fmt.Println("\nПопытка альтернативного метода...")
		
		// Если browser не работает, попробуем без него (может сработать в некоторых случаях)
		config.UseBrowser = false
		client2, err2 := ozoner.New(config)
		if err2 == nil {
			defer client2.Close()
			products, err = client2.SearchProducts(ctx, "техника", &ozoner.SearchFilters{
				Page:     1,
				PageSize: 4,
			})
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось получить товары: %v\n\nПримечание: Ozon.ru использует антибот-защиту. "+
			"Для работы может потребоваться:\n"+
			"1. Установленный Chrome/Chromium для browser client\n"+
			"2. Использование прокси\n"+
			"3. Настройка дополнительных параметров\n", err)
		os.Exit(1)
	}

	if len(products) == 0 {
		fmt.Println("⚠ Товары не найдены или не удалось распарсить результаты")
		fmt.Println("\nЭто может быть связано с:")
		fmt.Println("- Изменением структуры API Ozon.ru")
		fmt.Println("- Антибот-защитой")
		fmt.Println("- Необходимостью улучшения парсеров")
		return
	}

	fmt.Printf("✓ Найдено товаров: %d\n\n", len(products))
	fmt.Println("=== РЕЗУЛЬТАТЫ ПАРСИНГА ===\n")

	// Выводим информацию о товарах
	for i, product := range products {
		if i >= 4 {
			break
		}
		fmt.Printf("Товар #%d:\n", i+1)
		fmt.Printf("  ID: %s\n", product.ID)
		fmt.Printf("  Название: %s\n", product.Name)
		fmt.Printf("  Цена: %.2f %s\n", product.Price, product.Currency)
		if product.OldPrice != nil {
			fmt.Printf("  Старая цена: %.2f %s\n", *product.OldPrice, product.Currency)
		}
		if product.Rating > 0 {
			fmt.Printf("  Рейтинг: %.1f\n", product.Rating)
		}
		if product.ReviewsCount > 0 {
			fmt.Printf("  Отзывов: %d\n", product.ReviewsCount)
		}
		if product.SKU != "" {
			fmt.Printf("  SKU: %s\n", product.SKU)
		}
		if product.URL != "" {
			fmt.Printf("  URL: %s\n", product.URL)
		}
		if len(product.Images) > 0 {
			fmt.Printf("  Изображений: %d\n", len(product.Images))
		}
		fmt.Println()
	}

	// Сохраняем результаты в JSON
	jsonData, err := json.MarshalIndent(products, "", "  ")
	if err == nil {
		fmt.Println("=== JSON данные ===\n")
		fmt.Println(string(jsonData))
	}

	fmt.Println("\n=== Парсинг завершен успешно! ===")
}

