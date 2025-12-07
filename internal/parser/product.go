package parser

import (
	"fmt"

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

	if name, ok := getString(productData, "name", "title", "productName"); ok {
		product.Name = name
	}

	if desc, ok := getString(productData, "description", "fullDescription", "text"); ok {
		product.Description = desc
	}

	// Extract price information
	if price, ok := getFloat(productData, "price", "finalPrice", "priceWithDiscount", "currentPrice"); ok {
		product.Price = price
	}

	if oldPrice, ok := getFloat(productData, "oldPrice", "originalPrice", "priceWithoutDiscount"); ok {
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

	// Extract category
	if category, ok := getString(productData, "category", "categoryName", "categoryPath"); ok {
		product.Category = category
	}

	// Extract brand
	if brand, ok := getString(productData, "brand", "brandName", "manufacturer"); ok {
		product.Brand = brand
	}

	// Extract seller information
	if sellerID, ok := getString(productData, "sellerId", "seller_id", "sellerID", "vendorId"); ok {
		product.SellerID = sellerID
	}

	if sellerName, ok := getString(productData, "sellerName", "seller_name", "vendorName", "vendor"); ok {
		product.SellerName = sellerName
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

	// Try widgetStates
	if widgets, ok := data["widgetStates"].([]interface{}); ok {
		for _, widget := range widgets {
			if widgetMap, ok := widget.(map[string]interface{}); ok {
				// Look for product widget
				if product, ok := widgetMap["product"].(map[string]interface{}); ok {
					return product
				}
				// Look in widget data
				if data, ok := widgetMap["data"].(map[string]interface{}); ok {
					if product, ok := data["product"].(map[string]interface{}); ok {
						return product
					}
				}
			}
		}
	}

	// Try root level if it looks like product data
	if id, ok := data["id"].(string); ok && id != "" {
		return data
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


