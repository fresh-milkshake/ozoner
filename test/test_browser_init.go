package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"ozoner/internal/client"
)

func main() {
	fmt.Println("=== Testing Browser Client Initialization ===")

	config := client.DefaultBrowserClientConfig()
	config.Headless = true
	config.Timeout = 30 * time.Second

	fmt.Println("Creating browser client...")
	browserClient, err := client.NewBrowserClient(config)
	if err != nil {
		fmt.Printf("ERROR: Failed to create browser client: %v\n", err)
		os.Exit(1)
	}
	defer browserClient.Close()

	fmt.Println("✓ Browser client created successfully")

	// Test navigation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Testing navigation to about:blank...")
	if err := browserClient.Navigate(ctx, "about:blank"); err != nil {
		fmt.Printf("ERROR: Navigation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Navigation successful")

	// Test HTML extraction
	fmt.Println("Testing HTML extraction...")
	html, err := browserClient.GetHTML(ctx)
	if err != nil {
		fmt.Printf("ERROR: HTML extraction failed: %v\n", err)
		os.Exit(1)
	}

	if len(html) == 0 {
		fmt.Println("WARNING: HTML is empty")
	} else {
		fmt.Printf("✓ HTML extracted successfully (%d bytes)\n", len(html))
	}

	fmt.Println("\n=== All tests passed ===")
}
