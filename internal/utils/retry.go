package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts      int
	InitialDelay     time.Duration
	MaxDelay         time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// ShouldRetry determines if an error should trigger a retry
func ShouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		// Network errors should be retried
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		if _, ok := err.(*net.OpError); ok {
			return true
		}
		return false
	}

	if resp == nil {
		return false
	}

	// Retry on these status codes
	status := resp.StatusCode
	return status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

// RetryFunc is a function that can be retried
type RetryFunc func() error

// DoWithRetry executes a function with retry logic
func DoWithRetry(ctx context.Context, fn RetryFunc, config RetryConfig) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * config.BackoffMultiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("max retry attempts reached: %w", lastErr)
}

func isRetryableError(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

