package main

import (
	"context"
	"fmt"
	"log"
	"time"

	ozoner "ozoner/pkg/ozoner"
)

func main() {
	// Create client with custom configuration
	config := ozoner.Config{
		RateLimit:     5.0,  // 5 requests per second
		RetryAttempts: 3,
		Timeout:       60 * time.Second,
		UseBrowser:    false, // Use API by default
		WorkerCount:   10,    // 10 concurrent workers
		Headers: map[string]string{
			"Accept":     "application/json",
			"Accept-Language": "ru-RU,ru;q=0.9",
		},
	}

	client, err := ozoner.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Example: Batch product retrieval
	fmt.Println("=== Batch Product Retrieval ===")
	productIDs := []string{
		"123456789",
		"987654321",
		"111222333",
	}

	products := make([]*ozoner.Product, 0, len(productIDs))
	for _, id := range productIDs {
		product, err := client.GetProduct(ctx, id)
		if err != nil {
			log.Printf("Failed to get product %s: %v", id, err)
			continue
		}
		products = append(products, product)
		fmt.Printf("Retrieved: %s (%.2f %s)\n", 
			product.Name, product.Price, product.Currency)
	}

	fmt.Printf("\nSuccessfully retrieved %d/%d products\n", 
		len(products), len(productIDs))

	// Example: Search with multiple pages
	fmt.Println("\n=== Multi-page Search ===")
	query := "смартфон"
	maxPages := 3

	for page := 1; page <= maxPages; page++ {
		filters := &ozoner.SearchFilters{
			Page:     page,
			PageSize: 20,
		}

		products, err := client.SearchProducts(ctx, query, filters)
		if err != nil {
			log.Printf("Page %d error: %v", page, err)
			break
		}

		if len(products) == 0 {
			fmt.Printf("No more products on page %d\n", page)
			break
		}

		fmt.Printf("Page %d: Found %d products\n", page, len(products))
		time.Sleep(1 * time.Second) // Rate limiting
	}
}

