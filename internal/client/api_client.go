package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

// APIClient handles API requests to Ozon
type APIClient struct {
	httpClient *HTTPClient
	baseURL    string
}

// NewAPIClient creates a new API client
func NewAPIClient(httpClient *HTTPClient) *APIClient {
	return &APIClient{
		httpClient: httpClient,
		baseURL:    "https://www.ozon.ru",
	}
}

// SearchProducts searches for products using the API
func (c *APIClient) SearchProducts(ctx context.Context, query string, filters map[string]interface{}) (map[string]interface{}, error) {
	// Try direct search page first
	searchURL := fmt.Sprintf("%s/search/", c.baseURL)

	params := url.Values{}
	params.Set("text", query)
	params.Set("from_global", "true")

	if filters != nil {
		if minPrice, ok := filters["min_price"].(float64); ok {
			params.Set("min_price", strconv.FormatFloat(minPrice, 'f', 0, 64))
		}
		if maxPrice, ok := filters["max_price"].(float64); ok {
			params.Set("max_price", strconv.FormatFloat(maxPrice, 'f', 0, 64))
		}
		if category, ok := filters["category"].(string); ok && category != "" {
			params.Set("category", category)
		}
		if page, ok := filters["page"].(int); ok && page > 0 {
			params.Set("page", strconv.Itoa(page))
		}
	}

	fullURL := searchURL + "?" + params.Encode()

	// Try to get the page and extract JSON data from it
	resp, err := c.httpClient.Get(ctx, fullURL)
	if err != nil {
		// If direct page fails, try API endpoint
		return c.searchProductsAPI(ctx, query, filters)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try API endpoint as fallback
		return c.searchProductsAPI(ctx, query, filters)
	}

	// Read HTML response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.searchProductsAPI(ctx, query, filters)
	}

	// Try to extract JSON from HTML (Ozon embeds data in script tags)
	htmlStr := string(body)

	// Look for JSON data in script tags
	// Ozon typically embeds data in window.__APP_STATE__ or similar
	// For now, return a structure that indicates we got HTML
	result := map[string]interface{}{
		"html": htmlStr,
		"url":  fullURL,
	}

	return result, nil
}

// searchProductsAPI tries to use the API endpoint
func (c *APIClient) searchProductsAPI(ctx context.Context, query string, filters map[string]interface{}) (map[string]interface{}, error) {
	log.Debug("Using API endpoint for search", "query", query)

	apiURL := fmt.Sprintf("%s/api/entrypoint-api.bx/page/json/v2", c.baseURL)

	params := url.Values{}
	searchPath := fmt.Sprintf("/search/?text=%s&from_global=true", url.QueryEscape(query))
	params.Set("url", searchPath)

	if filters != nil {
		if page, ok := filters["page"].(int); ok && page > 0 {
			params.Set("page", strconv.Itoa(page))
		}
	}

	fullURL := apiURL + "?" + params.Encode()
	log.Debug("API request URL", "url", fullURL)

	resp, err := c.httpClient.Get(ctx, fullURL)
	if err != nil {
		log.Error("API request failed", "err", err, "url", fullURL)
		return nil, fmt.Errorf("failed to search products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyPreview := string(body)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200]
		}
		log.Error("API returned error status", "status", resp.StatusCode, "body_preview", bodyPreview)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Error("Failed to decode JSON response", "err", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info("API search successful", "query", query, "response_keys", len(result))
	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetProductPage gets product page data
func (c *APIClient) GetProductPage(ctx context.Context, productID string) (map[string]interface{}, error) {
	productURL := fmt.Sprintf("%s/product/%s/", c.baseURL, productID)

	apiURL := fmt.Sprintf("%s/api/entrypoint-api.bx/page/json/v2", c.baseURL)
	params := url.Values{}
	params.Set("url", productURL)

	fullURL := apiURL + "?" + params.Encode()

	resp, err := c.httpClient.Get(ctx, fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get product page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get product failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetSellerPage gets seller page data
func (c *APIClient) GetSellerPage(ctx context.Context, sellerID string) (map[string]interface{}, error) {
	sellerURL := fmt.Sprintf("%s/seller/%s/", c.baseURL, sellerID)

	apiURL := fmt.Sprintf("%s/api/entrypoint-api.bx/page/json/v2", c.baseURL)
	params := url.Values{}
	params.Set("url", sellerURL)

	fullURL := apiURL + "?" + params.Encode()

	resp, err := c.httpClient.Get(ctx, fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get seller failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// ExtractProductIDFromURL extracts product ID from Ozon URL
func ExtractProductIDFromURL(urlStr string) string {
	// Ozon URLs are like: https://www.ozon.ru/product/iphone-15-pro-256gb-128gb-512gb-1tb-128gb-natural-titanium-123456789/
	parts := strings.Split(urlStr, "/")
	for i, part := range parts {
		if part == "product" && i+1 < len(parts) {
			// Product ID is usually at the end of the slug
			slug := parts[i+1]
			slugParts := strings.Split(slug, "-")
			if len(slugParts) > 0 {
				// Last part is usually the ID
				return slugParts[len(slugParts)-1]
			}
		}
	}
	return ""
}

// ExtractSellerIDFromURL extracts seller ID from Ozon URL
func ExtractSellerIDFromURL(urlStr string) string {
	parts := strings.Split(urlStr, "/")
	for i, part := range parts {
		if part == "seller" && i+1 < len(parts) {
			return strings.TrimSuffix(parts[i+1], "/")
		}
	}
	return ""
}
