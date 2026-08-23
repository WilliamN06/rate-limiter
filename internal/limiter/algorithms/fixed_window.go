package algorithms

import (
	"context"
	"fmt"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/storage"
)

// FixedWindowLimiter implements the fixed window rate limiting algorithm
type FixedWindowLimiter struct {
	store storage.Store
}

// NewFixedWindowLimiter creates a new fixed window limiter
func NewFixedWindowLimiter(store storage.Store) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		store: store,
	}
}

// Check implements the Limiter interface for fixed window
func (f *FixedWindowLimiter) Check(ctx context.Context, key string, limit int, window time.Duration) (limiter.CheckResult, error) {
	now := time.Now()
	windowSeconds := int64(window.Seconds())
	if windowSeconds == 0 {
		windowSeconds = 1
	}

	// Window alignment: floor to window boundary
	windowStart := (now.Unix() / windowSeconds) * windowSeconds
	windowKey := fmt.Sprintf("fw:%s:%d", key, windowStart)

	// Increment counter for this window
	count, err := f.store.Increment(ctx, windowKey, 1)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("fixed_window", "failed to increment counter", err)
	}

	// Calculate reset time (end of current window)
	resetAt := time.Unix(windowStart+windowSeconds, 0)

	// Check if limit is exceeded
	if count > int64(limit) {
		retryAfter := int(resetAt.Sub(now).Seconds())
		if retryAfter < 0 {
			retryAfter = 0
		}

		return limiter.CheckResult{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: retryAfter,
		}, nil
	}

	// Request is allowed
	remaining := limit - int(count)
	return limiter.CheckResult{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}