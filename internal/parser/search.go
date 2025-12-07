package parser

import (
	"encoding/json"
	"fmt"

	"ozoner/internal/models"

	"github.com/charmbracelet/log"
)

// ParseSearchResults parses search results from API response
func ParseSearchResults(data map[string]interface{}) ([]models.ProductSearchResult, error) {
	log.Debug("Parsing search results", "data_keys", len(data))

	// Check if we got HTML instead of JSON
	if _, ok := data["html"].(string); ok {
		log.Warn("Received HTML instead of JSON, cannot parse without browser client")
		return nil, fmt.Errorf("received HTML response instead of JSON - browser client required")
	}

	var results []models.ProductSearchResult

	// Navigate through the response structure
	// Ozon API structure can vary, so we need to handle different formats
	var widgets []interface{}

	// Try to get widgetStates in different formats
	if widgetsArray, ok := data["widgetStates"].([]interface{}); ok {
		widgets = widgetsArray
	} else if widgetsMap, ok := data["widgetStates"].(map[string]interface{}); ok {
		// widgetStates might be a map, try to extract array from it
		if items, ok := widgetsMap["items"].([]interface{}); ok {
			widgets = items
		} else if states, ok := widgetsMap["states"].([]interface{}); ok {
			widgets = states
		} else {
			// Try to convert map values to array
			widgets = []interface{}{widgetsMap}
		}
	} else {
		log.Debug("No widgetStates found, trying alternative structure")
		// Try alternative structure
		if items, ok := extractProductsFromState(data); ok {
			log.Info("Products extracted from alternative structure", "count", len(items))
			return items, nil
		}
		// Try to find products directly in data
		if items, ok := data["items"].([]interface{}); ok {
			log.Debug("Found items directly in data", "count", len(items))
			var results []models.ProductSearchResult
			for _, item := range items {
				if product := parseProductItem(item); product != nil {
					results = append(results, *product)
				}
			}
			if len(results) > 0 {
				return results, nil
			}
		}
		log.Error("Invalid search response structure", "available_keys", getKeys(data))
		return nil, fmt.Errorf("invalid search response structure - no widgetStates or products found")
	}

	log.Debug("Found widgetStates", "widget_count", len(widgets))

	for i, widget := range widgets {
		widgetMap, ok := widget.(map[string]interface{})
		if !ok {
			log.Debug("Skipping invalid widget", "index", i)
			continue
		}

		// Look for product grid or list widgets
		if items := extractProductsFromWidget(widgetMap); len(items) > 0 {
			log.Debug("Found products in widget", "widget_index", i, "count", len(items))
			results = append(results, items...)
		}
	}

	// If no results from widgets, try direct extraction
	if len(results) == 0 {
		log.Debug("No results from widgets, trying direct extraction")
		if items, ok := extractProductsFromState(data); ok {
			log.Info("Products extracted from direct structure", "count", len(items))
			return items, nil
		}
		log.Warn("No products found in response")
	}

	log.Info("Parsing completed", "products_count", len(results))
	return results, nil
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractProductsFromWidget extracts products from a widget
func extractProductsFromWidget(widget map[string]interface{}) []models.ProductSearchResult {
	var results []models.ProductSearchResult

	// Try to find items in various possible locations
	items := findItemsInMap(widget, []string{"items", "products", "goods", "catalogItems"})

	for _, item := range items {
		if product := parseProductItem(item); product != nil {
			results = append(results, *product)
		}
	}

	return results
}

// extractProductsFromState extracts products from state structure
func extractProductsFromState(data map[string]interface{}) ([]models.ProductSearchResult, bool) {
	var results []models.ProductSearchResult

	// Try multiple possible paths
	paths := [][]string{
		{"catalog", "items"},
		{"searchResults", "items"},
		{"items"},
		{"products"},
	}

	for _, path := range paths {
		if items := findItemsByPath(data, path); len(items) > 0 {
			for _, item := range items {
				if product := parseProductItem(item); product != nil {
					results = append(results, *product)
				}
			}
			if len(results) > 0 {
				return results, true
			}
		}
	}

	return nil, false
}

// parseProductItem parses a single product item
func parseProductItem(item interface{}) *models.ProductSearchResult {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	product := &models.ProductSearchResult{}

	// Extract ID
	if id, ok := getString(itemMap, "id", "sku", "productId", "itemId"); ok {
		product.ID = id
	} else {
		return nil // ID is required
	}

	// Extract name
	if name, ok := getString(itemMap, "name", "title", "productName"); ok {
		product.Name = name
	}

	// Extract price
	if price, ok := getFloat(itemMap, "price", "finalPrice", "priceWithDiscount"); ok {
		product.Price = price
	}

	// Extract old price
	if oldPrice, ok := getFloat(itemMap, "oldPrice", "originalPrice", "priceWithoutDiscount"); ok {
		product.OldPrice = &oldPrice
	}

	// Extract currency
	if currency, ok := getString(itemMap, "currency", "currencyCode"); ok {
		product.Currency = currency
	} else {
		product.Currency = "RUB" // Default
	}

	// Extract rating
	if rating, ok := getFloat(itemMap, "rating", "averageRating", "ratingValue"); ok {
		product.Rating = rating
	}

	// Extract reviews count
	if reviewsCount, ok := getInt(itemMap, "reviewsCount", "reviewCount", "reviews"); ok {
		product.ReviewsCount = reviewsCount
	}

	// Extract image
	if image, ok := getString(itemMap, "image", "imageUrl", "thumbnail", "mainImage"); ok {
		product.Image = image
	}

	// Extract URL
	if url, ok := getString(itemMap, "url", "link", "href"); ok {
		product.URL = url
	} else if product.ID != "" {
		// Construct URL from ID
		product.URL = fmt.Sprintf("https://www.ozon.ru/product/%s/", product.ID)
	}

	// Extract SKU
	if sku, ok := getString(itemMap, "sku", "article", "vendorCode"); ok {
		product.SKU = sku
	}

	// Extract in stock status
	if inStock, ok := getBool(itemMap, "inStock", "available", "isAvailable"); ok {
		product.InStock = inStock
	} else {
		product.InStock = true // Default assumption
	}

	return product
}

// ParseSearchFilters converts SearchFilters to API parameters
func ParseSearchFilters(filters *models.SearchFilters) map[string]interface{} {
	if filters == nil {
		return nil
	}

	params := make(map[string]interface{})

	if filters.MinPrice != nil {
		params["min_price"] = *filters.MinPrice
	}
	if filters.MaxPrice != nil {
		params["max_price"] = *filters.MaxPrice
	}
	if filters.Category != "" {
		params["category"] = filters.Category
	}
	if filters.Brand != "" {
		params["brand"] = filters.Brand
	}
	if filters.Rating != nil {
		params["rating"] = *filters.Rating
	}
	if filters.InStock != nil {
		params["in_stock"] = *filters.InStock
	}
	if filters.SortBy != "" {
		params["sort_by"] = filters.SortBy
	}
	if filters.Page > 0 {
		params["page"] = filters.Page
	}
	if filters.PageSize > 0 {
		params["page_size"] = filters.PageSize
	}

	return params
}

// ParseSearchResponseJSON parses JSON string response
func ParseSearchResponseJSON(jsonStr string) ([]models.ProductSearchResult, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return ParseSearchResults(data)
}
