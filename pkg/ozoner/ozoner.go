package ozoner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ozoner/internal/client"
	"ozoner/internal/models"
	"ozoner/internal/parser"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/log"
)

// Type aliases for public API
type Product = models.Product
type Seller = models.Seller
type Review = models.Review
type ReviewResponse = models.ReviewResponse
type SearchFilters = models.SearchFilters

// Config holds configuration for the Ozoner client
type Config struct {
	// Rate limiting
	RateLimit float64

	// Retry configuration
	RetryAttempts int

	// Timeouts
	Timeout time.Duration

	// Browser configuration
	UseBrowser bool
	Headless   bool

	// HTTP client configuration
	UserAgent string
	Headers   map[string]string

	// Worker pool configuration
	WorkerCount int
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		RateLimit:     5.0, // Lower rate limit to avoid detection
		RetryAttempts: 3,
		Timeout:       30 * time.Second,
		UseBrowser:    false, // Use API by default, browser only when needed
		Headless:      true,
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Headers: map[string]string{
			"Accept":          "application/json, text/plain, */*",
			"Accept-Language": "ru-RU,ru;q=0.9",
		},
		WorkerCount: 5,
	}
}

// Ozoner is the main client for interacting with Ozon
type Ozoner struct {
	httpClient    *client.HTTPClient
	apiClient     *client.APIClient
	browserClient *client.BrowserClient
	config        Config
}

// New creates a new Ozoner client
func New(config Config) (*Ozoner, error) {
	log.Info("Creating new Ozoner client",
		"rate_limit", config.RateLimit,
		"use_browser", config.UseBrowser,
		"timeout", config.Timeout)

	if config.RateLimit <= 0 {
		config.RateLimit = 10.0
		log.Debug("Using default rate limit", "rate_limit", config.RateLimit)
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
		log.Debug("Using default timeout", "timeout", config.Timeout)
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 5
		log.Debug("Using default worker count", "workers", config.WorkerCount)
	}

	// Create HTTP client
	log.Debug("Creating HTTP client")
	httpClientConfig := client.DefaultHTTPClientConfig()
	httpClientConfig.RateLimit = config.RateLimit
	httpClientConfig.RetryAttempts = config.RetryAttempts
	httpClientConfig.Timeout = config.Timeout
	httpClientConfig.UserAgent = config.UserAgent
	httpClientConfig.Headers = config.Headers

	httpClient := client.NewHTTPClient(httpClientConfig)
	log.Debug("HTTP client created")

	// Create API client
	log.Debug("Creating API client")
	apiClient := client.NewAPIClient(httpClient)

	// Create browser client if needed
	var browserClient *client.BrowserClient
	if config.UseBrowser {
		log.Info("Initializing browser client")
		browserConfig := client.DefaultBrowserClientConfig()
		browserConfig.Headless = config.Headless
		browserConfig.UserAgent = config.UserAgent
		browserConfig.Timeout = config.Timeout

		var err error
		browserClient, err = client.NewBrowserClient(browserConfig)
		if err != nil {
			log.Error("Failed to create browser client", "err", err)
			return nil, fmt.Errorf("failed to create browser client: %w", err)
		}
		log.Info("Browser client created successfully")
	} else {
		log.Debug("Browser client disabled")
	}

	log.Info("Ozoner client created successfully")
	return &Ozoner{
		httpClient:    httpClient,
		apiClient:     apiClient,
		browserClient: browserClient,
		config:        config,
	}, nil
}

// SearchProducts searches for products by query with optional filters
func (o *Ozoner) SearchProducts(ctx context.Context, query string, filters *models.SearchFilters) ([]models.Product, error) {
	log.Info("Searching products", "query", query, "filters", filters)

	// Convert filters to API parameters
	filterParams := parser.ParseSearchFilters(filters)
	if filterParams == nil {
		filterParams = make(map[string]interface{})
	}
	log.Debug("Filter parameters", "params", filterParams)

	// Try API first if browser is not forced
	useBrowserFirst := o.config.UseBrowser
	if !useBrowserFirst {
		log.Debug("Requesting products from API")
		data, err := o.apiClient.SearchProducts(ctx, query, filterParams)
		if err != nil {
			log.Warn("API request failed", "err", err)
			// Check if it's a 403 error or anti-bot protection
			isForbidden := strings.Contains(err.Error(), "403") ||
				strings.Contains(err.Error(), "forbidden") ||
				strings.Contains(err.Error(), "anti-bot")

			if isForbidden {
				log.Warn("API access blocked - anti-bot protection detected")
				if o.browserClient == nil {
					log.Error("Browser client not available for fallback")
					return nil, fmt.Errorf("API access blocked (anti-bot protection). Enable browser client: UseBrowser: true in config. Error: %w", err)
				}
				log.Info("Falling back to browser client")
				return o.searchProductsWithBrowser(ctx, query, filters)
			}

			// For other errors, try browser if available
			if o.browserClient != nil {
				log.Info("Falling back to browser client due to API error")
				return o.searchProductsWithBrowser(ctx, query, filters)
			}

			log.Error("No fallback available", "err", err)
			return nil, fmt.Errorf("failed to search products: %w", err)
		}

		log.Debug("API response received", "data_keys", len(data))

		// Parse search results
		log.Debug("Parsing search results")
		searchResults, err := parser.ParseSearchResults(data)
		if err != nil {
			log.Warn("Failed to parse search results, trying browser", "err", err)
			// If parsing fails and browser is available, try browser
			if o.browserClient != nil {
				return o.searchProductsWithBrowser(ctx, query, filters)
			}
			log.Error("Failed to parse search results", "err", err)
			return nil, fmt.Errorf("failed to parse search results: %w", err)
		}

		log.Info("Search results parsed", "count", len(searchResults))

		// Check if we got valid results
		if len(searchResults) == 0 && o.browserClient != nil {
			log.Warn("No results from API, trying browser")
			return o.searchProductsWithBrowser(ctx, query, filters)
		}

		// Convert search results to full products
		products := make([]models.Product, 0, len(searchResults))
		for i, result := range searchResults {
			product := models.Product{
				ID:           result.ID,
				Name:         result.Name,
				Price:        result.Price,
				OldPrice:     result.OldPrice,
				Currency:     result.Currency,
				Rating:       result.Rating,
				ReviewsCount: result.ReviewsCount,
				Images:       []string{result.Image},
				URL:          result.URL,
				SKU:          result.SKU,
				InStock:      result.InStock,
			}
			products = append(products, product)
			log.Debug("Product converted", "index", i, "id", product.ID, "name", product.Name)
		}

		log.Info("Search completed successfully", "products_count", len(products))
		return products, nil
	}

	// Use browser directly if configured
	log.Info("Using browser client directly")
	return o.searchProductsWithBrowser(ctx, query, filters)
}

// searchProductsWithBrowser performs search using browser
func (o *Ozoner) searchProductsWithBrowser(ctx context.Context, query string, filters *models.SearchFilters) ([]models.Product, error) {
	if o.browserClient == nil {
		return nil, fmt.Errorf("browser client not available")
	}

	// Build search URL with pagination if needed
	searchURL := fmt.Sprintf("https://www.ozon.ru/search/?text=%s&from_global=true", query)
	if filters != nil && filters.Page > 1 {
		searchURL = fmt.Sprintf("%s&page=%d", searchURL, filters.Page)
	}

	// Use retry logic for browser operations
	maxRetries := 2

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Info("Retrying browser navigation", "attempt", attempt+1, "max_retries", maxRetries)
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(2+attempt) * time.Second):
			}
		}

		log.Debug("Starting browser navigation", "url", searchURL, "attempt", attempt+1)

		// Navigate to search page - use the provided context
		if err := o.browserClient.Navigate(ctx, searchURL); err != nil {
			log.Warn("Browser navigation failed", "err", err, "url", searchURL, "attempt", attempt+1)

			// Check if it's a context cancellation
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("navigation timeout or canceled: %w", err)
			}

			// Retry on other errors
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed to navigate after %d attempts: %w", maxRetries, err)
		}

		log.Info("Browser navigation successful", "url", searchURL)
		break
	}

	// Check context before proceeding
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	// Wait for results to load - increased wait for dynamic content
	log.Debug("Waiting for page content to load")
	time.Sleep(2 * time.Second)

	// Scroll multiple times to trigger lazy loading and load more products
	// Check context before scrolling
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled: %w", ctx.Err())
	default:
		// Scroll down multiple times to load more products
		for i := 0; i < 3; i++ {
			if err := o.browserClient.ScrollToBottom(ctx); err != nil {
				log.Warn("Failed to scroll page", "attempt", i+1, "err", err)
			}
			time.Sleep(1 * time.Second) // Wait between scrolls
		}
		// Final human-like scroll
		if err := o.browserClient.HumanLikeScroll(ctx); err != nil {
			log.Warn("Failed to do human-like scroll", "err", err)
		}
	}

	// Wait for dynamic content to load after scrolling
	time.Sleep(2 * time.Second)

	// Try to extract JSON data from page first - improved script
	script := `(function() {
		try {
			var result = {
				states: [],
				productData: null,
				widgetStates: null,
				scripts: [],
				url: window.location.href
			};
			
			// Try window states
			if (window.__APP_STATE__) {
				result.states.push({name: '__APP_STATE__', data: window.__APP_STATE__});
			}
			if (window.__INITIAL_STATE__) {
				result.states.push({name: '__INITIAL_STATE__', data: window.__INITIAL_STATE__});
			}
			if (window.__NEXT_DATA__) {
				result.states.push({name: '__NEXT_DATA__', data: window.__NEXT_DATA__});
			}
			if (window.widgetStates) {
				result.widgetStates = window.widgetStates;
			}
			
			// Try to find data in script tags
			try {
				var allScripts = document.querySelectorAll('script');
				for (var i = 0; i < allScripts.length; i++) {
					var script = allScripts[i];
					var text = script.textContent || script.innerHTML;
					
					// Look for JSON data in script content - improved extraction
					if (text.includes('widgetStates') || text.includes('searchResults') || text.includes('items') || text.includes('catalog')) {
						try {
							// Try multiple patterns to extract JSON
							var patterns = [
								/window\.__APP_STATE__\s*=\s*(\{[\s\S]*?\});/,
								/window\.widgetStates\s*=\s*(\{[\s\S]*?\});/,
								/var\s+widgetStates\s*=\s*(\{[\s\S]*?\});/,
								/"widgetStates"\s*:\s*(\[[\s\S]*?\]|\{[\s\S]*?\})/,
								/\{[\s\S]{500,50000}"widgetStates"[\s\S]{500,50000}\}/
							];
							
							for (var p = 0; p < patterns.length; p++) {
								var match = text.match(patterns[p]);
								if (match && match[1]) {
									try {
										var data = JSON.parse(match[1]);
										if (data && (data.widgetStates || data.items || data.products || data.catalog)) {
											result.productData = data;
											break;
										}
									} catch(e) {
										// Try to fix common JSON issues
										try {
											var fixed = match[1].replace(/,\s*}/g, '}').replace(/,\s*]/g, ']');
											var data = JSON.parse(fixed);
											if (data && (data.widgetStates || data.items || data.products || data.catalog)) {
												result.productData = data;
												break;
											}
										} catch(e2) {}
									}
								}
							}
							
							// If no match found, try simple pattern
							if (!result.productData) {
								var jsonMatch = text.match(/\{[\s\S]*"widgetStates"[\s\S]*\}/);
								if (jsonMatch) {
									try {
										var data = JSON.parse(jsonMatch[0]);
										if (data && (data.widgetStates || data.items || data.products || data.catalog)) {
											result.productData = data;
											break;
										}
									} catch(e) {}
								}
							}
						} catch(e) {}
					}
					
					// Try application/json scripts
					if (script.type === 'application/json' || script.getAttribute('type') === 'application/json') {
						try {
							var data = JSON.parse(text);
							if (data && (data.widgetStates || data.items || data.products || data.catalog)) {
								result.productData = data;
								break;
							}
						} catch(e) {}
					}
				}
			} catch(e) {}
			
			// Try to find data in data attributes
			try {
				var dataElements = document.querySelectorAll('[data-state], [data-initial-state], [data-widget-state]');
				for (var i = 0; i < Math.min(dataElements.length, 5); i++) {
					var el = dataElements[i];
					var stateData = el.getAttribute('data-state') || el.getAttribute('data-initial-state') || el.getAttribute('data-widget-state');
					if (stateData) {
						try {
							var data = JSON.parse(stateData);
							if (data && (data.widgetStates || data.items || data.products)) {
								result.productData = data;
								break;
							}
						} catch(e) {}
					}
				}
			} catch(e) {}
			
			return JSON.stringify(result);
		} catch(e) {
			return JSON.stringify({error: e.message, stack: e.stack});
		}
	})()`

	// Check context before extracting data
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	// Try to extract JSON data - use the provided context
	jsonData, extractErr := o.browserClient.ExtractJSONData(ctx, script)
	if extractErr == nil && jsonData != "" && jsonData != "null" {
		// Try to parse JSON data
		var extractedData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &extractedData); err == nil {
			// Try to find product data
			var dataToParse map[string]interface{}

			if productData, ok := extractedData["productData"].(map[string]interface{}); ok && productData != nil {
				dataToParse = productData
			} else if states, ok := extractedData["states"].([]interface{}); ok {
				// Try each state
				for _, state := range states {
					if stateMap, ok := state.(map[string]interface{}); ok {
						if stateData, ok := stateMap["data"].(map[string]interface{}); ok {
							dataToParse = stateData
							break
						}
					}
				}
			}

			// Try to parse from productData first
			if dataToParse != nil {
				// Parse search results from JSON
				searchResults, err := parser.ParseSearchResults(dataToParse)
				if err == nil && len(searchResults) > 0 {
					log.Info("Products parsed from JSON", "count", len(searchResults))
					// Convert to products
					products := make([]models.Product, 0, len(searchResults))
					for _, result := range searchResults {
						product := models.Product{
							ID:           result.ID,
							Name:         result.Name,
							Price:        result.Price,
							OldPrice:     result.OldPrice,
							Currency:     result.Currency,
							Rating:       result.Rating,
							ReviewsCount: result.ReviewsCount,
							Images:       []string{result.Image},
							URL:          result.URL,
							SKU:          result.SKU,
							InStock:      result.InStock,
						}
						products = append(products, product)
					}
					if len(products) > 0 {
						return products, nil
					}
				} else if err != nil {
					log.Debug("Failed to parse JSON products", "err", err)
				}
			}

			// Also try to parse from widgetStates directly if available
			if widgetStates, ok := extractedData["widgetStates"]; ok && widgetStates != nil {
				if widgetStatesMap, ok := widgetStates.(map[string]interface{}); ok {
					searchResults, err := parser.ParseSearchResults(widgetStatesMap)
					if err == nil && len(searchResults) > 0 {
						log.Info("Products parsed from widgetStates", "count", len(searchResults))
						products := make([]models.Product, 0, len(searchResults))
						for _, result := range searchResults {
							product := models.Product{
								ID:           result.ID,
								Name:         result.Name,
								Price:        result.Price,
								OldPrice:     result.OldPrice,
								Currency:     result.Currency,
								Rating:       result.Rating,
								ReviewsCount: result.ReviewsCount,
								Images:       []string{result.Image},
								URL:          result.URL,
								SKU:          result.SKU,
								InStock:      result.InStock,
							}
							products = append(products, product)
						}
						if len(products) > 0 {
							return products, nil
						}
					}
				}
			}
		}
	}

	// Check context before HTML parsing
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	// Fallback: try to parse HTML using goquery
	log.Debug("Trying HTML parsing as fallback")
	html, err := o.browserClient.GetHTML(ctx)
	if err == nil && len(html) > 0 {
		log.Info("HTML extracted, parsing products", "html_size", len(html))
		// Try to extract products from HTML using CSS selectors
		products := o.parseProductsFromHTML(html)
		if len(products) > 0 {
			log.Info("Products extracted from HTML", "count", len(products))
			return products, nil
		}
		log.Warn("No products found in HTML")
	} else {
		log.Warn("Failed to get HTML", "err", err)
	}

	return nil, fmt.Errorf("browser-based parsing: could not extract product data. "+
		"JSON extraction: %v, HTML length: %d bytes", extractErr, len(html))
}

// GetProduct retrieves detailed information about a product
func (o *Ozoner) GetProduct(ctx context.Context, productID string) (*models.Product, error) {
	// Get product page from API
	data, err := o.apiClient.GetProductPage(ctx, productID)
	if err != nil {
		// If API fails and browser is available, try browser
		if o.browserClient != nil {
			return o.getProductWithBrowser(ctx, productID)
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	// Parse product data
	product, err := parser.ParseProduct(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse product: %w", err)
	}

	return product, nil
}

// getProductWithBrowser retrieves product using browser
func (o *Ozoner) getProductWithBrowser(ctx context.Context, productID string) (*models.Product, error) {
	if o.browserClient == nil {
		return nil, fmt.Errorf("browser client not available")
	}

	productURL := fmt.Sprintf("https://www.ozon.ru/product/%s/", productID)

	log.Info("Navigating to product page", "url", productURL)
	if err := o.browserClient.Navigate(ctx, productURL); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page to load
	log.Debug("Waiting for page to load")
	time.Sleep(3 * time.Second)

	// Scroll to ensure content is loaded
	if err := o.browserClient.ScrollToBottom(ctx); err != nil {
		log.Warn("Failed to scroll page", "err", err)
	}
	time.Sleep(1 * time.Second)

	// Enhanced script to extract product data from page
	script := `(function() {
		try {
			var result = {
				states: [],
				productData: null,
				widgetStates: null,
				url: window.location.href
			};
			
			// Try window states
			if (window.__APP_STATE__) {
				result.states.push({name: '__APP_STATE__', data: window.__APP_STATE__});
			}
			if (window.__INITIAL_STATE__) {
				result.states.push({name: '__INITIAL_STATE__', data: window.__INITIAL_STATE__});
			}
			if (window.__NEXT_DATA__) {
				result.states.push({name: '__NEXT_DATA__', data: window.__NEXT_DATA__});
			}
			if (window.widgetStates) {
				result.widgetStates = window.widgetStates;
			}
			
			// Try to find product data in script tags
			try {
				var allScripts = document.querySelectorAll('script');
				for (var i = 0; i < allScripts.length; i++) {
					var script = allScripts[i];
					var text = script.textContent || script.innerHTML;
					
					// Look for JSON data in script content
					if (text.includes('widgetStates') || text.includes('product') || text.includes('webProduct')) {
						try {
							// Try multiple patterns to extract JSON
							var patterns = [
								/window\.__APP_STATE__\s*=\s*(\{[\s\S]*?\});/,
								/window\.widgetStates\s*=\s*(\{[\s\S]*?\});/,
								/var\s+widgetStates\s*=\s*(\{[\s\S]*?\});/,
								/"widgetStates"\s*:\s*(\[[\s\S]*?\]|\{[\s\S]*?\})/,
								/\{[\s\S]{500,100000}"widgetStates"[\s\S]{500,100000}\}/
							];
							
							for (var p = 0; p < patterns.length; p++) {
								var match = text.match(patterns[p]);
								if (match && match[1]) {
									try {
										var data = JSON.parse(match[1]);
										if (data && (data.widgetStates || data.product || data.webProductHeading || data.webProductPrice)) {
											result.productData = data;
											break;
										}
									} catch(e) {
										// Try to fix common JSON issues
										try {
											var fixed = match[1].replace(/,\s*}/g, '}').replace(/,\s*]/g, ']');
											var data = JSON.parse(fixed);
											if (data && (data.widgetStates || data.product || data.webProductHeading || data.webProductPrice)) {
												result.productData = data;
												break;
											}
										} catch(e2) {}
									}
								}
							}
						} catch(e) {}
					}
					
					// Try application/json scripts
					if (script.type === 'application/json' || script.getAttribute('type') === 'application/json') {
						try {
							var data = JSON.parse(text);
							if (data && (data.widgetStates || data.product || data.webProductHeading)) {
								result.productData = data;
								break;
							}
						} catch(e) {}
					}
				}
			} catch(e) {}
			
			return JSON.stringify(result);
		} catch(e) {
			return JSON.stringify({error: e.message, stack: e.stack});
		}
	})()`

	// Extract JSON data
	log.Debug("Extracting JSON data from product page")
	jsonData, extractErr := o.browserClient.ExtractJSONData(ctx, script)
	if extractErr != nil {
		log.Warn("Failed to extract JSON data, trying HTML parsing", "err", extractErr)
		// Fallback to HTML parsing
		return o.parseProductFromHTML(ctx, productID)
	}

	if jsonData == "" || jsonData == "null" {
		log.Warn("No JSON data extracted, trying HTML parsing")
		return o.parseProductFromHTML(ctx, productID)
	}

	// Parse extracted JSON data
	var extractedData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &extractedData); err != nil {
		log.Warn("Failed to parse extracted JSON, trying HTML parsing", "err", err)
		return o.parseProductFromHTML(ctx, productID)
	}

	// Try to find product data in extracted structure
	var dataToParse map[string]interface{}

	if productData, ok := extractedData["productData"].(map[string]interface{}); ok && productData != nil {
		dataToParse = productData
	} else if states, ok := extractedData["states"].([]interface{}); ok {
		// Try each state
		for _, state := range states {
			if stateMap, ok := state.(map[string]interface{}); ok {
				if stateData, ok := stateMap["data"].(map[string]interface{}); ok {
					dataToParse = stateData
					break
				}
			}
		}
	} else if widgetStates, ok := extractedData["widgetStates"]; ok && widgetStates != nil {
		if widgetStatesMap, ok := widgetStates.(map[string]interface{}); ok {
			dataToParse = widgetStatesMap
		} else if widgetStatesArray, ok := widgetStates.([]interface{}); ok {
			// Try to find product in widget states array
			for _, widget := range widgetStatesArray {
				if widgetMap, ok := widget.(map[string]interface{}); ok {
					if product, ok := widgetMap["product"].(map[string]interface{}); ok {
						dataToParse = product
						break
					}
					if data, ok := widgetMap["data"].(map[string]interface{}); ok {
						if product, ok := data["product"].(map[string]interface{}); ok {
							dataToParse = product
							break
						}
					}
				}
			}
		}
	}

	// If we found data, try to parse it
	if dataToParse != nil {
		log.Debug("Found product data in JSON, parsing")
		product, err := parser.ParseProduct(dataToParse)
		if err == nil && product != nil {
			// Ensure product ID is set
			if product.ID == "" {
				product.ID = productID
			}
			if product.URL == "" {
				product.URL = productURL
			}
			log.Info("Product parsed successfully from JSON", "id", product.ID, "name", product.Name)
			return product, nil
		}
		log.Warn("Failed to parse product from JSON data", "err", err)
	}

	// Fallback to HTML parsing
	log.Info("Falling back to HTML parsing")
	return o.parseProductFromHTML(ctx, productID)
}

// parseProductFromHTML parses product data from HTML using goquery
func (o *Ozoner) parseProductFromHTML(ctx context.Context, productID string) (*models.Product, error) {
	log.Debug("Parsing product from HTML", "product_id", productID)

	html, err := o.browserClient.GetHTML(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTML: %w", err)
	}

	if len(html) == 0 {
		return nil, fmt.Errorf("empty HTML content")
	}

	log.Debug("HTML extracted", "html_size", len(html))

	product := &models.Product{
		ID:         productID,
		URL:        fmt.Sprintf("https://www.ozon.ru/product/%s/", productID),
		Currency:   "RUB",
		Attributes: make(map[string]interface{}),
		InStock:    true, // Default assumption
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract name from h1 or product heading widget
	nameSelectors := []string{
		"h1",
		"[data-widget='webProductHeading'] h1",
		"[data-widget='webProductHeading']",
		".product-title",
		"[class*='product'][class*='title']",
		"[class*='heading']",
	}

	var name string
	for _, selector := range nameSelectors {
		name = strings.TrimSpace(doc.Find(selector).First().Text())
		if name != "" && !parser.IsPromoText(name) {
			break
		}
	}

	if name != "" {
		product.Name = name
		log.Debug("Extracted product name", "name", name)
	}

	// Extract description
	descSelectors := []string{
		"[data-widget='webProductDescription']",
		"[data-widget='webProductInfo']",
		".product-description",
		"[class*='description']",
		"[class*='Description']",
	}

	var description string
	for _, selector := range descSelectors {
		description = strings.TrimSpace(doc.Find(selector).First().Text())
		if description != "" {
			break
		}
	}

	if description != "" {
		product.Description = description
		log.Debug("Extracted product description", "length", len(description))
	}

	// Extract price
	priceSelectors := []string{
		"[data-widget='webProductPrice']",
		"[class*='price'][class*='final']",
		"[class*='Price']",
		".product-price",
	}

	var priceText string
	for _, selector := range priceSelectors {
		priceText = strings.TrimSpace(doc.Find(selector).First().Text())
		if priceText != "" {
			break
		}
	}

	if priceText != "" {
		if price, ok := parser.CleanPriceText(priceText); ok {
			product.Price = price
			log.Debug("Extracted product price", "price", price)
		}
	}

	// Extract rating
	ratingSelectors := []string{
		"[data-widget='webProductRating']",
		"[class*='rating']",
		"[data-rating]",
	}

	for _, selector := range ratingSelectors {
		ratingText := strings.TrimSpace(doc.Find(selector).First().Text())
		if ratingText == "" {
			ratingText = doc.Find(selector).First().AttrOr("data-rating", "")
		}
		if ratingText != "" {
			// Extract number from rating text
			ratingMatch := regexp.MustCompile(`(\d+\.?\d*)`).FindString(ratingText)
			if ratingMatch != "" {
				if rating, err := strconv.ParseFloat(ratingMatch, 64); err == nil && rating > 0 && rating <= 5 {
					product.Rating = rating
					log.Debug("Extracted product rating", "rating", rating)
					break
				}
			}
		}
	}

	// Extract images
	imgSelectors := []string{
		"[data-widget='webProductGallery'] img",
		".product-gallery img",
		"[class*='gallery'] img",
		"img[src*='multimedia']",
	}

	var images []string
	seenImages := make(map[string]bool)
	for _, selector := range imgSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			imgSrc, exists := s.Attr("src")
			if !exists {
				imgSrc, exists = s.Attr("data-src")
			}
			if exists && imgSrc != "" && !seenImages[imgSrc] {
				if !strings.HasPrefix(imgSrc, "http") {
					if strings.HasPrefix(imgSrc, "//") {
						imgSrc = "https:" + imgSrc
					} else {
						imgSrc = "https://www.ozon.ru" + imgSrc
					}
				}
				images = append(images, imgSrc)
				seenImages[imgSrc] = true
			}
		})
		if len(images) > 0 {
			break
		}
	}

	if len(images) > 0 {
		product.Images = images
		log.Debug("Extracted product images", "count", len(images))
	}

	// Extract brand and category from breadcrumbs or attributes
	breadcrumbSelectors := []string{
		"[data-widget='webBreadCrumbs'] a",
		".breadcrumbs a",
		"[class*='breadcrumb'] a",
	}

	var breadcrumbs []string
	doc.Find(strings.Join(breadcrumbSelectors, ", ")).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && text != "Главная" && text != "Каталог" {
			breadcrumbs = append(breadcrumbs, text)
		}
	})

	if len(breadcrumbs) > 0 {
		// Last breadcrumb is usually category
		product.Category = breadcrumbs[len(breadcrumbs)-1]
		// Brand might be in breadcrumbs too
		for _, bc := range breadcrumbs {
			if !strings.Contains(strings.ToLower(bc), "категория") && !strings.Contains(strings.ToLower(bc), "каталог") {
				product.Brand = bc
				break
			}
		}
	}

	log.Info("Product parsed from HTML", "id", product.ID, "name", product.Name, "has_description", product.Description != "", "has_price", product.Price > 0)
	return product, nil
}

// GetProductReviews retrieves reviews for a product
func (o *Ozoner) GetProductReviews(ctx context.Context, productID string, page int) (*models.ReviewResponse, error) {
	// Ozon reviews are typically loaded via API on the product page
	// First get the product page
	data, err := o.apiClient.GetProductPage(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product page: %w", err)
	}

	// Try to find reviews in the product page data
	reviewResponse, err := parser.ParseReviews(data)
	if err != nil {
		// Reviews might not be in initial page load
		// Return empty response with error info
		return &models.ReviewResponse{
			Reviews: []models.Review{},
			Page:    page,
		}, fmt.Errorf("reviews not found in initial page load: %w", err)
	}

	reviewResponse.Page = page
	return reviewResponse, nil
}

// GetSeller retrieves information about a seller
func (o *Ozoner) GetSeller(ctx context.Context, sellerID string) (*models.Seller, error) {
	// Get seller page from API
	data, err := o.apiClient.GetSellerPage(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller: %w", err)
	}

	// Parse seller data
	seller, err := parser.ParseSeller(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse seller: %w", err)
	}

	return seller, nil
}

// GetSellerProducts retrieves all products from a seller
func (o *Ozoner) GetSellerProducts(ctx context.Context, sellerID string, page int) ([]models.Product, error) {
	// Get seller page from API
	data, err := o.apiClient.GetSellerPage(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller page: %w", err)
	}

	// Parse seller products
	products, err := parser.ParseSellerProducts(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse seller products: %w", err)
	}

	return products, nil
}

// Close closes the client and releases resources
func (o *Ozoner) Close() error {
	if o.browserClient != nil {
		return o.browserClient.Close()
	}
	return nil
}

// ExtractProductIDFromURL extracts product ID from Ozon URL
func ExtractProductIDFromURL(url string) string {
	return client.ExtractProductIDFromURL(url)
}

// ExtractSellerIDFromURL extracts seller ID from Ozon URL
func ExtractSellerIDFromURL(url string) string {
	return client.ExtractSellerIDFromURL(url)
}

// parseProductsFromHTML extracts products from HTML using goquery
func (o *Ozoner) parseProductsFromHTML(html string) []models.Product {
	log.Debug("Parsing products from HTML", "html_size", len(html))

	var products []models.Product

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Error("Failed to parse HTML", "err", err)
		return products
	}

	log.Debug("HTML parsed successfully")

	// First, try to find all product links to get an idea of how many products are on the page
	allProductLinks := doc.Find("a[href*='/product/']")
	log.Info("Found product links in HTML", "count", allProductLinks.Length())

	// Ozon uses various selectors for products
	// Try common patterns with more variations - improved selectors
	selectors := []string{
		// Modern Ozon selectors - most specific first
		"[data-widget='searchResultsV2'] > div > div > div",
		"[data-widget='searchResultsV2'] .tile-root",
		"[data-widget='searchResultsV2'] > div > div",
		"div[data-widget='searchResultsV2'] .tile-root",
		"div[data-widget='searchResultsV2'] > div",
		// Alternative selectors
		".tile-root",
		"[data-test-id='tile-root']",
		"[data-widget='searchResultsV2'] a[href*='/product/']",
		".product-card",
		"[data-widget='searchResultsV2']",
		// Generic product containers - more specific
		"a[href*='/product/'][href*='-']", // Links with product slugs
		"a[href*='/product/']",
		"[data-product-id]",
		// Grid layouts
		".grid-item",
		".catalog-item",
		".product-item",
		// List layouts
		"li[data-product-id]",
		".product-list-item",
		// Additional Ozon-specific selectors
		"[class*='tile'][class*='root']",
		"[class*='product'][class*='card']",
		"[class*='catalog'][class*='item']",
	}

	var foundElements int
	maxProducts := 50                 // Increased limit
	seenURLs := make(map[string]bool) // Track seen products to avoid duplicates

	for _, selector := range selectors {
		log.Debug("Trying selector", "selector", selector)
		selectorFound := false

		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if len(products) >= maxProducts {
				return
			}
			foundElements++
			selectorFound = true

			product := models.Product{
				Attributes: make(map[string]interface{}),
			}

			// First, try to get URL to check for duplicates
			var href string
			var exists bool
			href, exists = s.Find("a[href*='/product/']").First().Attr("href")
			if !exists {
				href, exists = s.Closest("a[href*='/product/']").Attr("href")
			}
			if !exists {
				href = s.AttrOr("href", "")
				if href != "" {
					exists = true
				}
			}
			if !exists {
				href = s.AttrOr("data-href", "")
				if href != "" {
					exists = true
				}
			}

			// Normalize URL
			if exists && href != "" {
				if !strings.HasPrefix(href, "http") {
					href = "https://www.ozon.ru" + href
				}
				// Check for duplicates
				if seenURLs[href] {
					return // Skip duplicate
				}
				seenURLs[href] = true
				product.URL = href
				product.ID = ExtractProductIDFromURL(href)
			}

			// Extract name - try multiple selectors - improved
			nameSelectors := []string{
				"a[href*='/product/']",
				"a[href*='/product/'] span",
				"a[href*='/product/'] div",
				".tile-title",
				"[data-test-id='tile-title']",
				"[class*='title']",
				"span[title]",
				".title",
				"[data-test-id='product-title']",
				"h3",
				"h4",
				".product-title",
				"a[title]",
				"[class*='name']",
				"[class*='Name']",
			}

			var name string
			var nameFound bool
			for _, ns := range nameSelectors {
				name = strings.TrimSpace(s.Find(ns).First().Text())
				if name == "" {
					name = strings.TrimSpace(s.Find(ns).First().AttrOr("title", ""))
				}
				// Filter out promo texts
				if name != "" && !parser.IsPromoText(name) {
					nameFound = true
					break
				}
			}

			// If still no good name, try parent but filter promo texts
			if !nameFound {
				name = strings.TrimSpace(s.Closest("a[href*='/product/']").AttrOr("title", ""))
				if name != "" && !parser.IsPromoText(name) {
					nameFound = true
				}
			}

			// Try to exclude elements with promo badge classes
			if !nameFound {
				// Check if element has promo badge classes - if so, skip it
				hasPromoBadge := s.HasClass("promo") || s.HasClass("badge") ||
					s.Find(".promo, .badge, [class*='promo'], [class*='badge']").Length() > 0
				if hasPromoBadge {
					log.Debug("Skipping element with promo badge", "index", i)
					return
				}
			}

			// Don't skip if we have URL - we can use URL as name fallback
			if !nameFound && product.URL == "" {
				log.Debug("Skipping element without name or URL", "index", i)
				return
			}
			if nameFound {
				product.Name = name
				log.Debug("Found product name", "name", name)
			} else if product.URL != "" {
				// Use URL slug as name fallback
				parts := strings.Split(product.URL, "/")
				if len(parts) > 0 {
					slug := parts[len(parts)-1]
					slug = strings.TrimSuffix(slug, "/")
					slug = strings.ReplaceAll(slug, "-", " ")
					product.Name = slug
				}
			}

			// Extract price - try multiple selectors - improved
			priceSelectors := []string{
				".tile-price",
				"[data-test-id='tile-price']",
				"[class*='price'][class*='tile']",
				"[class*='Price']",
				".price",
				"span[data-test-id='tile-price']",
				"[data-test-id='price']",
				".product-price",
				".price-value",
				"span.price",
				".cost",
				"[class*='cost']",
				"[class*='Cost']",
				// Look for elements containing price symbols
				"span:contains('₽')",
				"div:contains('₽')",
				"span:contains('руб')",
			}

			var priceText string
			for _, ps := range priceSelectors {
				// Skip :contains selector as goquery doesn't support it directly
				if strings.Contains(ps, ":contains") {
					// Try to find elements with price symbols manually
					s.Find("span, div").Each(func(i int, el *goquery.Selection) {
						text := el.Text()
						if (strings.Contains(text, "₽") || strings.Contains(text, "руб")) && priceText == "" {
							priceText = text
						}
					})
					if priceText != "" {
						break
					}
					continue
				}
				priceText = s.Find(ps).First().Text()
				if priceText != "" {
					break
				}
			}

			// Try to find price in parent elements and siblings
			if priceText == "" {
				priceText = s.Closest("[data-widget]").Find(".tile-price, .price, [class*='price']").First().Text()
			}
			if priceText == "" {
				// Try siblings
				priceText = s.Siblings().Find(".tile-price, .price, [class*='price']").First().Text()
			}
			if priceText == "" {
				// Try parent's children
				priceText = s.Parent().Find(".tile-price, .price, [class*='price']").First().Text()
			}

			if priceText != "" {
				// Use CleanPriceText helper for consistent price parsing
				if price, ok := parser.CleanPriceText(priceText); ok {
					product.Price = price
					product.Currency = "RUB"
					log.Debug("Extracted price", "price", price, "original", priceText)
				} else {
					log.Debug("Failed to parse price", "text", priceText)
				}
			}

			// Extract image - try multiple attributes
			imgSelectors := []string{"img", "picture img", ".tile-image img", "[data-test-id='tile-image'] img"}
			var imgSrc string
			var imgExists bool

			for _, imgSel := range imgSelectors {
				imgSrc, imgExists = s.Find(imgSel).First().Attr("src")
				if !imgExists {
					imgSrc, imgExists = s.Find(imgSel).First().Attr("data-src")
				}
				if !imgExists {
					imgSrc, imgExists = s.Find(imgSel).First().Attr("data-lazy-src")
				}
				if !imgExists {
					imgSrc, imgExists = s.Find(imgSel).First().Attr("data-original")
				}
				if imgExists && imgSrc != "" {
					break
				}
			}

			if imgSrc != "" {
				if !strings.HasPrefix(imgSrc, "http") {
					if strings.HasPrefix(imgSrc, "//") {
						imgSrc = "https:" + imgSrc
					} else {
						imgSrc = "https://www.ozon.ru" + imgSrc
					}
				}
				product.Images = []string{imgSrc}
			}

			// Extract rating - improved selectors
			ratingSelectors := []string{
				".rating",
				"[data-test-id='rating']",
				"[class*='rating']",
				"[class*='Rating']",
				".product-rating",
				"[data-rating]",
				".stars",
				"[class*='star']",
				"[class*='Star']",
			}

			var ratingText string
			for _, rs := range ratingSelectors {
				ratingText = s.Find(rs).First().Text()
				if ratingText == "" {
					ratingText = s.Find(rs).First().AttrOr("data-rating", "")
				}
				if ratingText == "" {
					// Try aria-label
					ratingText = s.Find(rs).First().AttrOr("aria-label", "")
				}
				if ratingText != "" {
					break
				}
			}

			// Try to find rating in parent elements
			if ratingText == "" {
				ratingText = s.Closest("[data-widget]").Find(".rating, [class*='rating']").First().Text()
			}

			if ratingText != "" {
				// Clean rating text - improved
				// Extract number with optional decimal
				ratingMatch := regexp.MustCompile(`(\d+\.?\d*)`).FindString(ratingText)
				if ratingMatch != "" {
					if rating, err := strconv.ParseFloat(ratingMatch, 64); err == nil && rating > 0 && rating <= 5 {
						product.Rating = rating
						log.Debug("Extracted rating", "rating", rating)
					}
				}
			}

			// Extract reviews count - improved
			reviewsSelectors := []string{
				"[data-test-id='reviews-count']",
				"[class*='review'][class*='count']",
				"[class*='Review'][class*='Count']",
				".reviews-count",
				".review-count",
				"[class*='review']",
			}

			for _, rs := range reviewsSelectors {
				reviewsText := s.Find(rs).First().Text()
				if reviewsText == "" {
					// Try to find text containing "отзыв" or "review"
					s.Find("span, div").Each(func(i int, el *goquery.Selection) {
						text := strings.ToLower(el.Text())
						if (strings.Contains(text, "отзыв") || strings.Contains(text, "review")) && reviewsText == "" {
							reviewsText = el.Text()
						}
					})
				}
				if reviewsText != "" {
					// Extract number from text like "123 отзывов" or "123 reviews"
					reviewsMatch := regexp.MustCompile(`(\d+)`).FindString(reviewsText)
					if reviewsMatch != "" {
						if reviews, err := strconv.Atoi(reviewsMatch); err == nil && reviews > 0 {
							product.ReviewsCount = reviews
							log.Debug("Extracted reviews count", "count", reviews)
							break
						}
					}
				}
			}

			// Add product if it has at least name or URL
			// Extract description if available
			descSelectors := []string{
				".description",
				"[class*='description']",
				"[class*='Description']",
				".product-description",
				"[data-test-id='description']",
			}
			for _, ds := range descSelectors {
				desc := s.Find(ds).First().Text()
				if desc != "" {
					product.Description = strings.TrimSpace(desc)
					break
				}
			}

			if product.Name != "" || product.URL != "" {
				products = append(products, product)
				log.Debug("Product added", "name", product.Name, "url", product.URL, "price", product.Price, "rating", product.Rating, "has_image", len(product.Images) > 0)
			} else {
				log.Debug("Skipping product without name or URL", "index", i)
			}
		})

		if selectorFound {
			log.Debug("Selector found elements", "selector", selector, "products_so_far", len(products))
			// Continue to try other selectors to get more products
			// Don't break early - we want to collect as many products as possible
		}
	}

	log.Info("HTML parsing completed", "elements_found", foundElements, "products_extracted", len(products), "unique_urls", len(seenURLs))

	// If we found fewer products than expected, try a more aggressive approach
	if len(products) < 20 {
		log.Warn("Found fewer products than expected, trying alternative parsing approach", "current_count", len(products))
		// Try to extract products directly from links - more aggressive
		allProductLinks.Each(func(i int, link *goquery.Selection) {
			if len(products) >= maxProducts {
				return
			}

			href, exists := link.Attr("href")
			if !exists {
				return
			}

			// Filter out non-product links
			if !strings.Contains(href, "/product/") {
				return
			}

			// Normalize URL
			if !strings.HasPrefix(href, "http") {
				href = "https://www.ozon.ru" + href
			}

			// Check for duplicates
			if seenURLs[href] {
				return
			}
			seenURLs[href] = true

			// Try to find product container - more options
			container := link.Closest(".tile-root, [data-widget], [class*='tile'], [class*='product'], [class*='card'], [class*='item']")
			if container.Length() == 0 {
				container = link.Parent().Parent() // Go up two levels
			}
			if container.Length() == 0 {
				container = link.Parent()
			}

			product := models.Product{
				Attributes: make(map[string]interface{}),
				URL:        href,
				ID:         ExtractProductIDFromURL(href),
			}

			// Extract name from link - improved with promo text filtering
			var name string
			var nameFound bool

			name = strings.TrimSpace(link.Text())
			if name != "" && !parser.IsPromoText(name) {
				nameFound = true
			}

			if !nameFound {
				name = strings.TrimSpace(link.AttrOr("title", ""))
				if name != "" && !parser.IsPromoText(name) {
					nameFound = true
				}
			}

			if !nameFound {
				// Try to get from container with more selectors
				name = strings.TrimSpace(container.Find(".tile-title, [class*='title'], h3, h4, [class*='name'], span[title]").First().Text())
				if name != "" && !parser.IsPromoText(name) {
					nameFound = true
				}
			}

			if !nameFound {
				// Try aria-label
				name = strings.TrimSpace(link.AttrOr("aria-label", ""))
				if name != "" && !parser.IsPromoText(name) {
					nameFound = true
				}
			}

			if nameFound {
				product.Name = name
			}

			// Extract price from container - improved search
			priceText := container.Find("[class*='price'], .tile-price, .price, [class*='Price'], [class*='cost']").First().Text()
			if priceText == "" {
				// Try siblings
				priceText = link.Siblings().Find("[class*='price'], .price").First().Text()
			}
			if priceText == "" {
				// Try parent's siblings
				priceText = link.Parent().Siblings().Find("[class*='price'], .price").First().Text()
			}
			if priceText != "" {
				// Use CleanPriceText helper for consistent price parsing
				if price, ok := parser.CleanPriceText(priceText); ok {
					product.Price = price
					product.Currency = "RUB"
					log.Debug("Extracted price from link", "price", price)
				}
			}

			// Extract image - improved
			imgSrc, imgExists := container.Find("img").First().Attr("src")
			if !imgExists {
				imgSrc, imgExists = container.Find("img").First().Attr("data-src")
			}
			if !imgExists {
				imgSrc, imgExists = container.Find("img").First().Attr("data-lazy-src")
			}
			if !imgExists {
				// Try to find image near the link
				imgSrc, imgExists = link.Closest("div, article, section").Find("img").First().Attr("src")
			}
			if imgExists && imgSrc != "" {
				if !strings.HasPrefix(imgSrc, "http") {
					if strings.HasPrefix(imgSrc, "//") {
						imgSrc = "https:" + imgSrc
					} else {
						imgSrc = "https://www.ozon.ru" + imgSrc
					}
				}
				product.Images = []string{imgSrc}
			}

			// Extract rating - try to find near the link
			ratingText := container.Find("[class*='rating'], .rating, [data-rating]").First().Text()
			if ratingText == "" {
				ratingText = container.Find("[class*='rating'], .rating, [data-rating]").First().AttrOr("data-rating", "")
			}
			if ratingText != "" {
				ratingMatch := regexp.MustCompile(`(\d+\.?\d*)`).FindString(ratingText)
				if ratingMatch != "" {
					if rating, err := strconv.ParseFloat(ratingMatch, 64); err == nil && rating > 0 && rating <= 5 {
						product.Rating = rating
					}
				}
			}

			// Only add if we have at least name or URL
			if product.Name != "" || product.URL != "" {
				products = append(products, product)
				log.Debug("Product added from link", "name", product.Name, "price", product.Price, "rating", product.Rating)
			}
		})
		log.Info("Alternative parsing completed", "total_products", len(products))
	}

	log.Info("HTML parsing completed", "elements_found", foundElements, "products_extracted", len(products), "unique_urls", len(seenURLs))
	return products
}
