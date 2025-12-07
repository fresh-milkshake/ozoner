package main

import (
	"context"
	"fmt"
	"log"
	"time"

	ozoner "ozoner/pkg/ozoner"
)

func main() {
	// Create client with default configuration
	client, err := ozoner.New(ozoner.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example 1: Simple search
	fmt.Println("=== Simple Search ===")
	products, err := client.SearchProducts(ctx, "iphone", nil)
	if err != nil {
		log.Printf("Search error: %v", err)
	} else {
		fmt.Printf("Found %d products\n", len(products))
		for i, product := range products {
			if i >= 5 { // Show only first 5
				break
			}
			fmt.Printf("  - %s: %.2f %s\n", product.Name, product.Price, product.Currency)
		}
	}

	// Example 2: Search with filters
	fmt.Println("\n=== Search with Filters ===")
	filters := &ozoner.SearchFilters{
		MinPrice: floatPtr(10000),
		MaxPrice: floatPtr(100000),
		InStock:  boolPtr(true),
		SortBy:   "price_asc",
		Page:     1,
		PageSize: 20,
	}

	products, err = client.SearchProducts(ctx, "ноутбук", filters)
	if err != nil {
		log.Printf("Search with filters error: %v", err)
	} else {
		fmt.Printf("Found %d products with filters\n", len(products))
		for i, product := range products {
			if i >= 3 {
				break
			}
			fmt.Printf("  - %s: %.2f %s (Rating: %.1f)\n", 
				product.Name, product.Price, product.Currency, product.Rating)
		}
	}

	// Example 3: Search by category
	fmt.Println("\n=== Search by Category ===")
	categoryFilters := &ozoner.SearchFilters{
		Category: "electronics",
		Page:     1,
	}
	
	products, err = client.SearchProducts(ctx, "смартфон", categoryFilters)
	if err != nil {
		log.Printf("Category search error: %v", err)
	} else {
		fmt.Printf("Found %d products in category\n", len(products))
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

