package parser

import (
	"strconv"
	"strings"
)

// Helper functions for extracting values from maps

func getString(m map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case string:
				return v, true
			case float64:
				return strconv.FormatFloat(v, 'f', 0, 64), true
			case int:
				return strconv.Itoa(v), true
			}
		}
	}
	return "", false
}

func getFloat(m map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case float64:
				return v, true
			case int:
				return float64(v), true
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

func getInt(m map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case int:
				return v, true
			case float64:
				return int(v), true
			case string:
				if i, err := strconv.Atoi(v); err == nil {
					return i, true
				}
			}
		}
	}
	return 0, false
}

func getBool(m map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case bool:
				return v, true
			case string:
				return strings.ToLower(v) == "true" || v == "1", true
			case int:
				return v != 0, true
			}
		}
	}
	return false, false
}

func findItemsByPath(data map[string]interface{}, path []string) []interface{} {
	current := data
	for i, key := range path {
		if i == len(path)-1 {
			if val, ok := current[key]; ok {
				if items, ok := val.([]interface{}); ok {
					return items
				}
			}
		} else {
			if next, ok := current[key].(map[string]interface{}); ok {
				current = next
			} else {
				return nil
			}
		}
	}
	return nil
}

func findItemsInMap(m map[string]interface{}, keys []string) []interface{} {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if items, ok := val.([]interface{}); ok {
				return items
			}
		}
	}
	return nil
}

// IsPromoText checks if a text is a promotional text that should be filtered out
func IsPromoText(text string) bool {
	if text == "" {
		return false
	}
	text = strings.ToLower(strings.TrimSpace(text))

	// List of promotional text patterns
	promoPatterns := []string{
		"распродажа",
		"баллов за отзыв",
		"цена что надо",
		"до ",
		"вау-цены",
		"нет в наличии",
		"?at=",
		"морковок",
		"баллов",
		"за отзыв",
		"балл",
		"отзыв",
		"распродаж",
		"скидк",
		"акци",
		"промо",
		"спецпредложени",
		"выгод",
		"дешев",
	}

	for _, pattern := range promoPatterns {
		// Special handling for "до " - only match if followed by number or specific words
		if pattern == "до " {
			if strings.HasPrefix(text, "до ") {
				// Check if it's followed by a number or specific promo words
				after := text[3:] // Skip "до "
				if len(after) > 0 {
					// Check for number patterns like "до 8900", "до 8 900"
					hasNumber := false
					for _, r := range after[:min(len(after), 20)] {
						if r >= '0' && r <= '9' {
							hasNumber = true
							break
						}
					}
					if hasNumber && (strings.Contains(after, "балл") || strings.Contains(after, "руб") || len(after) < 30) {
						return true
					}
				}
			}
		} else if strings.Contains(text, pattern) {
			return true
		}
	}

	// Check if text is too short (likely a badge)
	if len(text) < 5 {
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidatePrice checks if a price value is reasonable
func ValidatePrice(price float64) bool {
	// Price should be positive and reasonable (not too large)
	// Ozon prices are typically in rubles, so reasonable range is 0.01 to 100,000,000
	if price <= 0 {
		return false
	}
	if price > 100000000 {
		return false
	}
	return true
}

// CleanPriceText cleans price text and converts to float
func CleanPriceText(priceText string) (float64, bool) {
	// Remove common price formatting
	priceText = strings.ReplaceAll(priceText, " ", "")
	priceText = strings.ReplaceAll(priceText, "\u00A0", "") // Non-breaking space
	priceText = strings.ReplaceAll(priceText, "₽", "")
	priceText = strings.ReplaceAll(priceText, "руб", "")
	priceText = strings.ReplaceAll(priceText, ",", ".")

	// Remove all non-digit characters except dot
	var cleaned strings.Builder
	dotFound := false
	for _, r := range priceText {
		if r >= '0' && r <= '9' {
			cleaned.WriteRune(r)
		} else if r == '.' && !dotFound {
			cleaned.WriteRune(r)
			dotFound = true
		}
	}

	priceStr := cleaned.String()
	if priceStr == "" {
		return 0, false
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, false
	}

	// Check if price might be in kopeks (if it's a very large number)
	if price > 1000000 {
		price = price / 100
	}

	if !ValidatePrice(price) {
		return 0, false
	}

	return price, true
}

// deepSearchInMap recursively searches for a value in nested map structures
func deepSearchInMap(data interface{}, keys []string, maxDepth int) interface{} {
	if maxDepth <= 0 {
		return nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		// Try direct access
		for _, key := range keys {
			if val, ok := v[key]; ok {
				return val
			}
		}

		// Recursively search in nested maps
		for _, val := range v {
			if result := deepSearchInMap(val, keys, maxDepth-1); result != nil {
				return result
			}
		}
	case []interface{}:
		// Search in array elements
		for _, item := range v {
			if result := deepSearchInMap(item, keys, maxDepth-1); result != nil {
				return result
			}
		}
	}

	return nil
}
