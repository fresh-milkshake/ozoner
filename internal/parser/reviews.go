package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ozoner/internal/models"
)

// ParseReviews parses reviews from API response
func ParseReviews(data map[string]interface{}) (*models.ReviewResponse, error) {
	response := &models.ReviewResponse{
		Reviews: []models.Review{},
	}

	// Try to find reviews in various locations
	reviews := findReviews(data)
	
	for _, reviewData := range reviews {
		if review := parseReviewItem(reviewData); review != nil {
			response.Reviews = append(response.Reviews, *review)
		}
	}

	// Extract pagination info
	if total, ok := getInt(data, "total", "totalCount", "totalReviews"); ok {
		response.Total = total
	}

	if page, ok := getInt(data, "page", "currentPage", "pageNumber"); ok {
		response.Page = page
	}

	if pages, ok := getInt(data, "pages", "totalPages", "pageCount"); ok {
		response.Pages = pages
	}

	return response, nil
}

// findReviews searches for reviews in the response
func findReviews(data map[string]interface{}) []interface{} {
	// Try direct access
	if reviews, ok := data["reviews"].([]interface{}); ok {
		return reviews
	}

	// Try items
	if items, ok := data["items"].([]interface{}); ok {
		return items
	}

	// Try widgetStates
	if widgets, ok := data["widgetStates"].([]interface{}); ok {
		for _, widget := range widgets {
			if widgetMap, ok := widget.(map[string]interface{}); ok {
				// Look for review widget
				if reviews, ok := widgetMap["reviews"].([]interface{}); ok {
					return reviews
				}
				if data, ok := widgetMap["data"].(map[string]interface{}); ok {
					if reviews, ok := data["reviews"].([]interface{}); ok {
						return reviews
					}
					if items, ok := data["items"].([]interface{}); ok {
						return items
					}
				}
			}
		}
	}

	return nil
}

// parseReviewItem parses a single review item
func parseReviewItem(item interface{}) *models.Review {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	review := &models.Review{}

	// Extract ID
	if id, ok := getString(itemMap, "id", "reviewId", "review_id"); ok {
		review.ID = id
	} else {
		// Generate ID from other fields if needed
		if author, ok := getString(itemMap, "author"); ok {
			if date, ok := getString(itemMap, "date", "createdAt"); ok {
				review.ID = fmt.Sprintf("%s_%s", author, date)
			}
		}
	}

	// Extract product ID
	if productID, ok := getString(itemMap, "productId", "product_id", "itemId"); ok {
		review.ProductID = productID
	}

	// Extract author
	if author, ok := getString(itemMap, "author", "userName", "username", "name", "reviewer"); ok {
		review.Author = author
	}

	// Extract rating (1-5)
	if rating, ok := getInt(itemMap, "rating", "score", "stars"); ok {
		if rating >= 1 && rating <= 5 {
			review.Rating = rating
		}
	} else if ratingFloat, ok := getFloat(itemMap, "rating", "score"); ok {
		review.Rating = int(ratingFloat)
		if review.Rating < 1 {
			review.Rating = 1
		}
		if review.Rating > 5 {
			review.Rating = 5
		}
	}

	// Extract title
	if title, ok := getString(itemMap, "title", "subject", "headline"); ok {
		review.Title = title
	}

	// Extract text
	if text, ok := getString(itemMap, "text", "content", "comment", "body", "message"); ok {
		review.Text = text
	}

	// Extract date
	if date, ok := getString(itemMap, "date", "createdAt", "created_at", "timestamp", "time"); ok {
		review.Date = date
		// Try to parse and format date
		if parsedDate, err := parseDate(date); err == nil {
			review.Date = parsedDate.Format("2006-01-02")
		}
	}

	// Extract verification status
	if verified, ok := getBool(itemMap, "verified", "isVerified", "verifiedPurchase"); ok {
		review.Verified = verified
	}

	// Extract helpful count
	if helpful, ok := getInt(itemMap, "helpful", "helpfulCount", "likes", "thumbsUp"); ok {
		review.Helpful = helpful
	}

	// Extract images
	if images := getReviewImages(itemMap); len(images) > 0 {
		review.Images = images
	}

	// Extract pros
	if pros := getStringArray(itemMap, "pros", "advantages", "positive"); len(pros) > 0 {
		review.Pros = pros
	}

	// Extract cons
	if cons := getStringArray(itemMap, "cons", "disadvantages", "negative"); len(cons) > 0 {
		review.Cons = cons
	}

	return review
}

// getReviewImages extracts image URLs from review
func getReviewImages(data map[string]interface{}) []string {
	var images []string

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

	return images
}

// getStringArray extracts string array from map
func getStringArray(m map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if arr, ok := val.([]interface{}); ok {
				var result []string
				for _, item := range arr {
					if str, ok := item.(string); ok {
						result = append(result, str)
					}
				}
				if len(result) > 0 {
					return result
				}
			} else if str, ok := val.(string); ok {
				// Single string, split by common delimiters
				if strings.Contains(str, ",") {
					parts := strings.Split(str, ",")
					var result []string
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if part != "" {
							result = append(result, part)
						}
					}
					return result
				}
				return []string{str}
			}
		}
	}
	return nil
}

// parseDate tries to parse various date formats
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02.01.2006",
		"02/01/2006",
		time.RFC1123,
		time.RFC1123Z,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	// Try Unix timestamp
	if timestamp, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
		// Check if it's in seconds or milliseconds
		if timestamp > 1e10 {
			return time.Unix(timestamp/1000, 0), nil
		}
		return time.Unix(timestamp, 0), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}


