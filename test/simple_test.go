package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ozoner "ozoner/pkg/ozoner"
)

func main() {
	// Открываем файл для логирования
	logFile, _ := os.Create("test_output.txt")
	defer logFile.Close()

	write := func(s string) {
		fmt.Print(s)
		fmt.Fprint(logFile, s)
		logFile.Sync()
	}

	write("=== Простой тест ===\n")
	write("Создание клиента...\n")

	config := ozoner.DefaultConfig()
	config.UseBrowser = false
	config.RateLimit = 3.0
	config.Timeout = 30 * time.Second

	client, err := ozoner.New(config)
	if err != nil {
		write(fmt.Sprintf("Ошибка: %v\n", err))
		os.Exit(1)
	}
	defer client.Close()

	write("Клиент создан\n")
	write("Поиск товаров...\n")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	products, err := client.SearchProducts(ctx, "техника", &ozoner.SearchFilters{
		Page:     1,
		PageSize: 5,
	})

	if err != nil {
		write(fmt.Sprintf("Ошибка поиска: %v\n", err))
		os.Exit(1)
	}

	write(fmt.Sprintf("Найдено товаров: %d\n", len(products)))
	for i, p := range products {
		write(fmt.Sprintf("%d. %s - %.2f %s\n", i+1, p.Name, p.Price, p.Currency))
	}

	write("Тест завершен\n")
}


