package parser

import (
	"fmt"
	"strings"

	"ozoner/internal/models"
)

// ParseProduct parses product data from API response
func ParseProduct(data map[string]interface{}) (*models.Product, error) {
	product := &models.Product{
		Attributes: make(map[string]interface{}),
	}

	// Try to find product data in various locations
	productData := findProductData(data)
	if productData == nil {
		return nil, fmt.Errorf("product data not found in response")
	}

	// Extract basic fields
	if id, ok := getString(productData, "id", "sku", "productId", "itemId"); ok {
		product.ID = id
	} else {
		return nil, fmt.Errorf("product ID not found")
	}

	// Extract name - filter promo texts
	nameKeys := []string{"name", "title", "productName", "heading", "productTitle", "itemName"}
	var name string
	var nameFound bool

	for _, key := range nameKeys {
		if n, ok := getString(productData, key); ok && n != "" {
			if !IsPromoText(n) {
				name = n
				nameFound = true
				break
			}
		}
	}

	// Try deep search if not found
	if !nameFound {
		if result := deepSearchInMap(productData, nameKeys, 3); result != nil {
			if nameStr, ok := result.(string); ok && nameStr != "" {
				if !IsPromoText(nameStr) {
					name = nameStr
					nameFound = true
				}
			}
		}
	}

	if nameFound {
		product.Name = strings.TrimSpace(name)
	}

	// Extract description - try multiple keys and deep search
	descKeys := []string{"description", "fullDescription", "text", "content", "about", "productDescription", "longDescription"}
	var desc string
	var descFound bool

	for _, key := range descKeys {
		if d, ok := getString(productData, key); ok && d != "" {
			desc = d
			descFound = true
			break
		}
	}

	// Try deep search if not found
	if !descFound {
		if result := deepSearchInMap(productData, descKeys, 3); result != nil {
			if descStr, ok := result.(string); ok && descStr != "" {
				desc = descStr
				descFound = true
			}
		}
	}

	if descFound {
		product.Description = strings.TrimSpace(desc)
	}

	// Extract price information - validate prices
	priceKeys := []string{"price", "finalPrice", "priceWithDiscount", "currentPrice", "salePrice"}
	var price float64
	var priceFound bool

	for _, key := range priceKeys {
		if p, ok := getFloat(productData, key); ok {
			if ValidatePrice(p) {
				price = p
				priceFound = true
				break
			}
		}
	}

	// Try price object structure
	if !priceFound {
		if priceObj, ok := productData["price"].(map[string]interface{}); ok {
			if p, ok := getFloat(priceObj, "value", "amount", "final", "current"); ok {
				if ValidatePrice(p) {
					price = p
					priceFound = true
				}
			}
		}
	}

	// Try deep search
	if !priceFound {
		if result := deepSearchInMap(productData, priceKeys, 3); result != nil {
			if priceVal, ok := result.(float64); ok {
				if ValidatePrice(priceVal) {
					price = priceVal
					priceFound = true
				}
			}
		}
	}

	if priceFound {
		product.Price = price
	}

	// Extract old price - validate
	oldPriceKeys := []string{"oldPrice", "originalPrice", "priceWithoutDiscount", "regularPrice"}
	var oldPrice float64
	var oldPriceFound bool

	for _, key := range oldPriceKeys {
		if op, ok := getFloat(productData, key); ok {
			if ValidatePrice(op) {
				oldPrice = op
				oldPriceFound = true
				break
			}
		}
	}

	if oldPriceFound {
		product.OldPrice = &oldPrice
	}

	if currency, ok := getString(productData, "currency", "currencyCode"); ok {
		product.Currency = currency
	} else {
		product.Currency = "RUB"
	}

	// Extract rating and reviews
	if rating, ok := getFloat(productData, "rating", "averageRating", "ratingValue"); ok {
		product.Rating = rating
	}

	if reviewsCount, ok := getInt(productData, "reviewsCount", "reviewCount", "reviews", "totalReviews"); ok {
		product.ReviewsCount = reviewsCount
	}

	// Extract images
	if images := getImages(productData); len(images) > 0 {
		product.Images = images
	} else if image, ok := getString(productData, "image", "imageUrl", "mainImage", "thumbnail"); ok {
		product.Images = []string{image}
	}

	// Extract URL
	if url, ok := getString(productData, "url", "link", "href", "webUrl"); ok {
		product.URL = url
	} else if product.ID != "" {
		product.URL = fmt.Sprintf("https://www.ozon.ru/product/%s/", product.ID)
	}

	// Extract SKU
	if sku, ok := getString(productData, "sku", "article", "vendorCode", "productSku"); ok {
		product.SKU = sku
	}

	// Extract category - try multiple keys and deep search
	categoryKeys := []string{"category", "categoryName", "categoryPath", "categoryTitle", "categoryId"}
	var category string
	var categoryFound bool

	for _, key := range categoryKeys {
		if c, ok := getString(productData, key); ok && c != "" {
			category = c
			categoryFound = true
			break
		}
	}

	// Try nested category structure
	if !categoryFound {
		if catObj, ok := productData["category"].(map[string]interface{}); ok {
			if c, ok := getString(catObj, "name", "title", "path"); ok && c != "" {
				category = c
				categoryFound = true
			}
		}
	}

	// Try deep search
	if !categoryFound {
		if result := deepSearchInMap(productData, categoryKeys, 3); result != nil {
			if catStr, ok := result.(string); ok && catStr != "" {
				category = catStr
				categoryFound = true
			}
		}
	}

	if categoryFound {
		product.Category = strings.TrimSpace(category)
	}

	// Extract brand - try multiple keys and deep search
	brandKeys := []string{"brand", "brandName", "manufacturer", "vendor", "producer"}
	var brand string
	var brandFound bool

	for _, key := range brandKeys {
		if b, ok := getString(productData, key); ok && b != "" {
			brand = b
			brandFound = true
			break
		}
	}

	// Try nested brand structure
	if !brandFound {
		if brandObj, ok := productData["brand"].(map[string]interface{}); ok {
			if b, ok := getString(brandObj, "name", "title"); ok && b != "" {
				brand = b
				brandFound = true
			}
		}
	}

	// Try deep search
	if !brandFound {
		if result := deepSearchInMap(productData, brandKeys, 3); result != nil {
			if brandStr, ok := result.(string); ok && brandStr != "" {
				brand = brandStr
				brandFound = true
			}
		}
	}

	if brandFound {
		product.Brand = strings.TrimSpace(brand)
	}

	// Extract seller information - try multiple keys and deep search
	sellerIDKeys := []string{"sellerId", "seller_id", "sellerID", "vendorId", "merchantId", "shopId"}
	var sellerID string
	var sellerIDFound bool

	for _, key := range sellerIDKeys {
		if sid, ok := getString(productData, key); ok && sid != "" {
			sellerID = sid
			sellerIDFound = true
			break
		}
	}

	// Try nested seller structure
	if !sellerIDFound {
		if sellerObj, ok := productData["seller"].(map[string]interface{}); ok {
			if sid, ok := getString(sellerObj, "id", "sellerId", "vendorId"); ok && sid != "" {
				sellerID = sid
				sellerIDFound = true
			}
		}
		if vendorObj, ok := productData["vendor"].(map[string]interface{}); ok {
			if sid, ok := getString(vendorObj, "id", "vendorId"); ok && sid != "" {
				sellerID = sid
				sellerIDFound = true
			}
		}
	}

	// Try deep search
	if !sellerIDFound {
		if result := deepSearchInMap(productData, sellerIDKeys, 3); result != nil {
			if sidStr, ok := result.(string); ok && sidStr != "" {
				sellerID = sidStr
				sellerIDFound = true
			}
		}
	}

	if sellerIDFound {
		product.SellerID = sellerID
	}

	// Extract seller name
	sellerNameKeys := []string{"sellerName", "seller_name", "vendorName", "vendor", "merchantName", "shopName"}
	var sellerName string
	var sellerNameFound bool

	for _, key := range sellerNameKeys {
		if sn, ok := getString(productData, key); ok && sn != "" {
			sellerName = sn
			sellerNameFound = true
			break
		}
	}

	// Try nested seller structure
	if !sellerNameFound {
		if sellerObj, ok := productData["seller"].(map[string]interface{}); ok {
			if sn, ok := getString(sellerObj, "name", "title", "sellerName"); ok && sn != "" {
				sellerName = sn
				sellerNameFound = true
			}
		}
		if vendorObj, ok := productData["vendor"].(map[string]interface{}); ok {
			if sn, ok := getString(vendorObj, "name", "title", "vendorName"); ok && sn != "" {
				sellerName = sn
				sellerNameFound = true
			}
		}
	}

	// Try deep search
	if !sellerNameFound {
		if result := deepSearchInMap(productData, sellerNameKeys, 3); result != nil {
			if snStr, ok := result.(string); ok && snStr != "" {
				sellerName = snStr
				sellerNameFound = true
			}
		}
	}

	if sellerNameFound {
		product.SellerName = strings.TrimSpace(sellerName)
	}

	// Extract stock status
	if inStock, ok := getBool(productData, "inStock", "available", "isAvailable", "hasStock"); ok {
		product.InStock = inStock
	} else {
		product.InStock = true // Default
	}

	// Extract attributes
	if attrs, ok := productData["attributes"].(map[string]interface{}); ok {
		product.Attributes = attrs
	} else if specs, ok := productData["specifications"].(map[string]interface{}); ok {
		product.Attributes = specs
	} else if characteristics, ok := productData["characteristics"].([]interface{}); ok {
		// Convert array of characteristics to map
		for _, char := range characteristics {
			if charMap, ok := char.(map[string]interface{}); ok {
				if name, ok := getString(charMap, "name", "title"); ok {
					if value, ok := getString(charMap, "value", "text"); ok {
						product.Attributes[name] = value
					}
				}
			}
		}
	}

	return product, nil
}

// findProductData searches for product data in the response
func findProductData(data map[string]interface{}) map[string]interface{} {
	// Try direct access
	if product, ok := data["product"].(map[string]interface{}); ok {
		return product
	}

	// Try widgetStates - expanded search
	if widgets, ok := data["widgetStates"].([]interface{}); ok {
		for _, widget := range widgets {
			if widgetMap, ok := widget.(map[string]interface{}); ok {
				// Look for product widget
				if product, ok := widgetMap["product"].(map[string]interface{}); ok {
					return product
				}
				// Look in widget data
				if widgetData, ok := widgetMap["data"].(map[string]interface{}); ok {
					if product, ok := widgetData["product"].(map[string]interface{}); ok {
						return product
					}
					// Try direct access if data looks like product
					if id, ok := widgetData["id"].(string); ok && id != "" {
						return widgetData
					}
				}
				// Try widget-specific keys
				widgetKeys := []string{"webProductHeading", "webProductPrice", "webProductInfo", "catalog", "productInfo"}
				for _, key := range widgetKeys {
					if widgetData, ok := widgetMap[key].(map[string]interface{}); ok {
						if product, ok := widgetData["product"].(map[string]interface{}); ok {
							return product
						}
						if id, ok := widgetData["id"].(string); ok && id != "" {
							return widgetData
						}
					}
				}
			}
		}
	}

	// Try widgetStates as map
	if widgetsMap, ok := data["widgetStates"].(map[string]interface{}); ok {
		// Try common widget keys
		widgetKeys := []string{"webProductHeading", "webProductPrice", "webProductInfo", "catalog", "product", "productInfo"}
		for _, key := range widgetKeys {
			if widget, ok := widgetsMap[key].(map[string]interface{}); ok {
				if product, ok := widget["product"].(map[string]interface{}); ok {
					return product
				}
				if data, ok := widget["data"].(map[string]interface{}); ok {
					if product, ok := data["product"].(map[string]interface{}); ok {
						return product
					}
					if id, ok := data["id"].(string); ok && id != "" {
						return data
					}
				}
				if id, ok := widget["id"].(string); ok && id != "" {
					return widget
				}
			}
		}
	}

	// Try root level if it looks like product data
	if id, ok := data["id"].(string); ok && id != "" {
		return data
	}

	// Try deep search for product data
	if result := deepSearchInMap(data, []string{"product", "item", "goods"}, 5); result != nil {
		if productMap, ok := result.(map[string]interface{}); ok {
			if id, ok := productMap["id"].(string); ok && id != "" {
				return productMap
			}
		}
	}

	return nil
}

// getImages extracts image URLs from product data
func getImages(data map[string]interface{}) []string {
	var images []string

	// Try different possible locations
	if imgArray, ok := data["images"].([]interface{}); ok {
		for _, img := range imgArray {
			if imgStr, ok := img.(string); ok {
				images = append(images, imgStr)
			} else if imgMap, ok := img.(map[string]interface{}); ok {
				if url, ok := getString(imgMap, "url", "src", "imageUrl", "link"); ok {
					images = append(images, url)
				}
			}
		}
	}

	if len(images) == 0 {
		// Try alternative keys
		for _, key := range []string{"imageUrls", "photos", "gallery", "media"} {
			if imgArray, ok := data[key].([]interface{}); ok {
				for _, img := range imgArray {
					if imgStr, ok := img.(string); ok {
						images = append(images, imgStr)
					} else if imgMap, ok := img.(map[string]interface{}); ok {
						if url, ok := getString(imgMap, "url", "src", "imageUrl", "link"); ok {
							images = append(images, url)
						}
					}
				}
				if len(images) > 0 {
					break
				}
			}
		}
	}

	return images
}

// ParseProductFromHTML extracts product data from HTML (for browser-based parsing)
func ParseProductFromHTML(html string) (*models.Product, error) {
	// This would use goquery to parse HTML
	// For now, return error as this requires HTML structure analysis
	return nil, fmt.Errorf("HTML parsing not yet implemented, use API method")
}
