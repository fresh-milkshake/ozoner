package parser

import (
	"fmt"

	"ozoner/internal/models"
)

// ParseSeller parses seller data from API response
func ParseSeller(data map[string]interface{}) (*models.Seller, error) {
	seller := &models.Seller{}

	// Try to find seller data in various locations
	sellerData := findSellerData(data)
	if sellerData == nil {
		return nil, fmt.Errorf("seller data not found in response")
	}

	// Extract ID
	if id, ok := getString(sellerData, "id", "sellerId", "seller_id", "vendorId"); ok {
		seller.ID = id
	} else {
		return nil, fmt.Errorf("seller ID not found")
	}

	// Extract name
	if name, ok := getString(sellerData, "name", "title", "sellerName", "vendorName"); ok {
		seller.Name = name
	}

	// Extract description
	if desc, ok := getString(sellerData, "description", "about", "info", "text"); ok {
		seller.Description = desc
	}

	// Extract rating
	if rating, ok := getFloat(sellerData, "rating", "averageRating", "ratingValue", "score"); ok {
		seller.Rating = rating
	}

	// Extract reviews count
	if reviewsCount, ok := getInt(sellerData, "reviewsCount", "reviewCount", "reviews", "totalReviews"); ok {
		seller.ReviewsCount = reviewsCount
	}

	// Extract products count
	if productsCount, ok := getInt(sellerData, "productsCount", "productCount", "itemsCount", "totalProducts"); ok {
		seller.ProductsCount = productsCount
	}

	// Extract URL
	if url, ok := getString(sellerData, "url", "link", "href", "webUrl"); ok {
		seller.URL = url
	} else if seller.ID != "" {
		seller.URL = fmt.Sprintf("https://www.ozon.ru/seller/%s/", seller.ID)
	}

	// Extract logo
	if logo, ok := getString(sellerData, "logo", "logoUrl", "image", "avatar", "icon"); ok {
		seller.Logo = logo
	}

	// Extract verification status
	if verified, ok := getBool(sellerData, "isVerified", "verified", "isTrusted", "trusted"); ok {
		seller.IsVerified = verified
	}

	// Extract joined date
	if joinedDate, ok := getString(sellerData, "joinedDate", "createdAt", "registrationDate", "since"); ok {
		seller.JoinedDate = joinedDate
	}

	return seller, nil
}

// ParseSellerProducts parses products from seller page
func ParseSellerProducts(data map[string]interface{}) ([]models.Product, error) {
	var products []models.Product

	// Try to find products in various locations
	items := findSellerProducts(data)
	
	for _, item := range items {
		if product, err := parseSellerProductItem(item); err == nil {
			products = append(products, *product)
		}
	}

	return products, nil
}

// findSellerData searches for seller data in the response
func findSellerData(data map[string]interface{}) map[string]interface{} {
	// Try direct access
	if seller, ok := data["seller"].(map[string]interface{}); ok {
		return seller
	}

	// Try vendor
	if vendor, ok := data["vendor"].(map[string]interface{}); ok {
		return vendor
	}

	// Try widgetStates
	if widgets, ok := data["widgetStates"].([]interface{}); ok {
		for _, widget := range widgets {
			if widgetMap, ok := widget.(map[string]interface{}); ok {
				if seller, ok := widgetMap["seller"].(map[string]interface{}); ok {
					return seller
				}
				if data, ok := widgetMap["data"].(map[string]interface{}); ok {
					if seller, ok := data["seller"].(map[string]interface{}); ok {
						return seller
					}
				}
			}
		}
	}

	// Try root level if it looks like seller data
	if id, ok := data["id"].(string); ok && id != "" {
		// Check if it's seller data (not product)
		if _, hasProducts := data["products"]; hasProducts {
			return data
		}
	}

	return nil
}

// findSellerProducts searches for products on seller page
func findSellerProducts(data map[string]interface{}) []interface{} {
	// Try various paths
	paths := [][]string{
		{"products", "items"},
		{"catalog", "items"},
		{"items"},
		{"goods"},
	}

	for _, path := range paths {
		if items := findItemsByPath(data, path); len(items) > 0 {
			return items
		}
	}

	// Try widgetStates
	if widgets, ok := data["widgetStates"].([]interface{}); ok {
		for _, widget := range widgets {
			if widgetMap, ok := widget.(map[string]interface{}); ok {
				if items := findItemsInMap(widgetMap, []string{"items", "products", "goods"}); len(items) > 0 {
					return items
				}
			}
		}
	}

	return nil
}

// parseSellerProductItem parses a product item from seller page
func parseSellerProductItem(item interface{}) (*models.Product, error) {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid product item format")
	}

	product := &models.Product{
		Attributes: make(map[string]interface{}),
	}

	// Extract ID
	if id, ok := getString(itemMap, "id", "sku", "productId"); ok {
		product.ID = id
	} else {
		return nil, fmt.Errorf("product ID not found")
	}

	// Extract name
	if name, ok := getString(itemMap, "name", "title"); ok {
		product.Name = name
	}

	// Extract price
	if price, ok := getFloat(itemMap, "price", "finalPrice"); ok {
		product.Price = price
	}

	if oldPrice, ok := getFloat(itemMap, "oldPrice", "originalPrice"); ok {
		product.OldPrice = &oldPrice
	}

	if currency, ok := getString(itemMap, "currency"); ok {
		product.Currency = currency
	} else {
		product.Currency = "RUB"
	}

	// Extract rating
	if rating, ok := getFloat(itemMap, "rating"); ok {
		product.Rating = rating
	}

	// Extract images
	if image, ok := getString(itemMap, "image", "imageUrl"); ok {
		product.Images = []string{image}
	}

	// Extract URL
	if url, ok := getString(itemMap, "url", "link"); ok {
		product.URL = url
	} else if product.ID != "" {
		product.URL = fmt.Sprintf("https://www.ozon.ru/product/%s/", product.ID)
	}

	// Extract SKU
	if sku, ok := getString(itemMap, "sku"); ok {
		product.SKU = sku
	}

	// Extract stock status
	if inStock, ok := getBool(itemMap, "inStock", "available"); ok {
		product.InStock = inStock
	} else {
		product.InStock = true
	}

	return product, nil
}


