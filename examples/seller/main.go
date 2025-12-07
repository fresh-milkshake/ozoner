package main

import (
	"context"
	"fmt"
	"log"
	"time"

	ozoner "ozoner/pkg/ozoner"
)

func main() {
	// Create client
	client, err := ozoner.New(ozoner.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example 1: Get seller information
	fmt.Println("=== Get Seller Information ===")
	sellerID := "seller-id" // Replace with actual seller ID
	seller, err := client.GetSeller(ctx, sellerID)
	if err != nil {
		log.Printf("Get seller error: %v", err)
	} else {
		fmt.Printf("Seller: %s\n", seller.Name)
		fmt.Printf("Rating: %.1f (%d reviews)\n", seller.Rating, seller.ReviewsCount)
		fmt.Printf("Products: %d\n", seller.ProductsCount)
		fmt.Printf("Verified: %v\n", seller.IsVerified)
		if seller.Description != "" {
			fmt.Printf("Description: %s...\n", truncate(seller.Description, 100))
		}
		if seller.Logo != "" {
			fmt.Printf("Logo: %s\n", seller.Logo)
		}
		fmt.Printf("URL: %s\n", seller.URL)
	}

	// Example 2: Extract seller ID from URL
	fmt.Println("\n=== Extract Seller ID from URL ===")
	url := "https://www.ozon.ru/seller/some-seller-name-12345/"
	extractedID := ozoner.ExtractSellerIDFromURL(url)
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Extracted ID: %s\n", extractedID)

	// Example 3: Get seller products
	fmt.Println("\n=== Get Seller Products ===")
	products, err := client.GetSellerProducts(ctx, sellerID, 1)
	if err != nil {
		log.Printf("Get seller products error: %v", err)
	} else {
		fmt.Printf("Found %d products\n", len(products))
		for i, product := range products {
			if i >= 5 {
				break
			}
			fmt.Printf("  %d. %s - %.2f %s\n", 
				i+1, product.Name, product.Price, product.Currency)
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

