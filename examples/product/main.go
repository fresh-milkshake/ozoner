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

	// Example 1: Get product by ID
	fmt.Println("=== Get Product by ID ===")
	productID := "123456789" // Replace with actual product ID
	product, err := client.GetProduct(ctx, productID)
	if err != nil {
		log.Printf("Get product error: %v", err)
	} else {
		fmt.Printf("Product: %s\n", product.Name)
		fmt.Printf("Price: %.2f %s\n", product.Price, product.Currency)
		if product.OldPrice != nil {
			fmt.Printf("Old Price: %.2f %s\n", *product.OldPrice, product.Currency)
		}
		fmt.Printf("Rating: %.1f (%d reviews)\n", product.Rating, product.ReviewsCount)
		fmt.Printf("In Stock: %v\n", product.InStock)
		fmt.Printf("URL: %s\n", product.URL)
		if len(product.Images) > 0 {
			fmt.Printf("Images: %d\n", len(product.Images))
		}
		if product.Description != "" {
			fmt.Printf("Description: %s...\n", truncate(product.Description, 100))
		}
	}

	// Example 2: Extract product ID from URL
	fmt.Println("\n=== Extract Product ID from URL ===")
	url := "https://www.ozon.ru/product/iphone-15-pro-256gb-128gb-512gb-1tb-128gb-natural-titanium-123456789/"
	extractedID := ozoner.ExtractProductIDFromURL(url)
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Extracted ID: %s\n", extractedID)

	// Example 3: Get product reviews
	fmt.Println("\n=== Get Product Reviews ===")
	reviews, err := client.GetProductReviews(ctx, productID, 1)
	if err != nil {
		log.Printf("Get reviews error: %v", err)
	} else {
		fmt.Printf("Total reviews: %d (Page %d of %d)\n", 
			reviews.Total, reviews.Page, reviews.Pages)
		for i, review := range reviews.Reviews {
			if i >= 3 {
				break
			}
			fmt.Printf("\nReview %d:\n", i+1)
			fmt.Printf("  Author: %s\n", review.Author)
			fmt.Printf("  Rating: %d/5\n", review.Rating)
			fmt.Printf("  Title: %s\n", review.Title)
			if review.Text != "" {
				fmt.Printf("  Text: %s...\n", truncate(review.Text, 100))
			}
			fmt.Printf("  Date: %s\n", review.Date)
			fmt.Printf("  Verified: %v\n", review.Verified)
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

