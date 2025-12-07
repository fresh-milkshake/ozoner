package client

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"

	"ozoner/internal/utils"

	"github.com/charmbracelet/log"
	"golang.org/x/net/publicsuffix"
)

// HTTPClient wraps http.Client with rate limiting and retry
type HTTPClient struct {
	client      *http.Client
	rateLimiter *utils.RateLimiter
	retryConfig utils.RetryConfig
	userAgent   string
	headers     map[string]string
}

// HTTPClientConfig holds configuration for HTTPClient
type HTTPClientConfig struct {
	RateLimit     float64
	RetryAttempts int
	Timeout       time.Duration
	UserAgent     string
	Headers       map[string]string
	Transport     http.RoundTripper
}

// DefaultHTTPClientConfig returns default configuration
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		RateLimit:     10.0,
		RetryAttempts: 3,
		Timeout:       30 * time.Second,
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Headers:       make(map[string]string),
	}
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(config HTTPClientConfig) *HTTPClient {
	if config.RateLimit <= 0 {
		config.RateLimit = 10.0
	}

	transport := config.Transport
	if transport == nil {
		transport = &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		}
	}

	retryConfig := utils.DefaultRetryConfig()
	retryConfig.MaxAttempts = config.RetryAttempts

	// Create cookie jar for session management
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		// Fallback if cookie jar creation fails
		jar = nil
	}

	// Create HTTP client with redirect limit and cookie jar
	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects to prevent infinite loops
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	return &HTTPClient{
		client:      httpClient,
		rateLimiter: utils.NewRateLimiter(config.RateLimit),
		retryConfig: retryConfig,
		userAgent:   config.UserAgent,
		headers:     config.Headers,
	}
}

// Get performs a GET request with rate limiting and retry
func (c *HTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, nil)
}

// Post performs a POST request with rate limiting and retry
func (c *HTTPClient) Post(ctx context.Context, url string, body io.Reader, contentType string) (*http.Response, error) {
	headers := map[string]string{
		"Content-Type": contentType,
	}
	return c.Do(ctx, http.MethodPost, url, body, headers)
}

// Do performs an HTTP request with rate limiting and retry
func (c *HTTPClient) Do(ctx context.Context, method, url string, body io.Reader, extraHeaders map[string]string) (*http.Response, error) {
	var resp *http.Response
	var err error
	attempt := 0

	log.Debug("Starting HTTP request", "method", method, "url", url)

	err = utils.DoWithRetry(ctx, func() error {
		attempt++
		if attempt > 1 {
			log.Debug("Retrying HTTP request", "attempt", attempt, "url", url)
		}

		// Wait for rate limiter
		log.Debug("Waiting for rate limiter", "url", url)
		if err := c.rateLimiter.Wait(ctx); err != nil {
			log.Error("Rate limiter wait failed", "err", err, "url", url)
			return err
		}
		log.Debug("Rate limiter passed", "url", url)

		// Create request
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			log.Error("Failed to create request", "err", err, "url", url)
			return err
		}

		// Set realistic headers to avoid detection
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
		req.Header.Set("Referer", "https://www.ozon.ru/")
		req.Header.Set("Origin", "https://www.ozon.ru")

		// Modern browser headers
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Sec-CH-UA", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("Sec-CH-UA-Mobile", "?0")
		req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)

		// Additional headers for better compatibility
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("DNT", "1")
		req.Header.Set("Cache-Control", "max-age=0")

		// Viewport and screen info (if available)
		req.Header.Set("Viewport-Width", "1920")

		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}

		log.Debug("Sending HTTP request", "method", method, "url", url, "headers_count", len(req.Header))

		// Perform request
		startTime := time.Now()
		resp, err = c.client.Do(req)
		duration := time.Since(startTime)

		if err != nil {
			log.Warn("HTTP request failed", "err", err, "url", url, "duration_ms", duration.Milliseconds())
			return err
		}

		log.Info("HTTP response received",
			"status", resp.StatusCode,
			"url", url,
			"duration_ms", duration.Milliseconds(),
			"content_length", resp.ContentLength,
			"content_type", resp.Header.Get("Content-Type"))

		// Handle gzip compression
		if resp.Header.Get("Content-Encoding") == "gzip" {
			log.Debug("Decompressing gzip response", "url", url)
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				log.Error("Failed to create gzip reader", "err", err, "url", url)
				return fmt.Errorf("failed to create gzip reader: %w", err)
			}
			resp.Body.Close()
			resp.Body = reader
		}

		// Check if should retry
		if utils.ShouldRetry(err, resp) {
			log.Warn("Retryable error detected", "status", resp.StatusCode, "url", url)
			if resp != nil {
				resp.Body.Close()
			}
			return fmt.Errorf("retryable error: status %d", resp.StatusCode)
		}

		// Check for 403 Forbidden (anti-bot protection)
		if resp.StatusCode == http.StatusForbidden {
			log.Error("Access forbidden - anti-bot protection", "status", 403, "url", url)
			resp.Body.Close()
			return fmt.Errorf("access forbidden (403) - anti-bot protection detected, consider using browser client")
		}

		if resp.StatusCode >= 400 {
			log.Warn("HTTP error status", "status", resp.StatusCode, "url", url)
		}

		return nil
	}, c.retryConfig)

	if err != nil {
		log.Error("HTTP request failed after retries", "err", err, "url", url, "attempts", attempt)
		return nil, err
	}

	log.Debug("HTTP request completed successfully", "url", url, "status", resp.StatusCode)
	return resp, nil
}

// SetHeader sets a header for all requests
func (c *HTTPClient) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// SetCookie sets a cookie for all requests
func (c *HTTPClient) SetCookie(name, value string) {
	// Cookies are typically set via headers or cookie jar
	c.SetHeader("Cookie", fmt.Sprintf("%s=%s", name, value))
}
