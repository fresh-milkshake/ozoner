package utils

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter wraps golang.org/x/time/rate.Limiter with convenience methods
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter
// rps is requests per second
func NewRateLimiter(rps float64) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), int(rps)),
	}
}

// Wait blocks until the limiter allows the request
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

// Allow checks if a request is allowed without blocking
func (rl *RateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// AllowN checks if n requests are allowed without blocking
func (rl *RateLimiter) AllowN(now time.Time, n int) bool {
	return rl.limiter.AllowN(now, n)
}

