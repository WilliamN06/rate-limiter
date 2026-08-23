package algorithms

import (
	"context"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/storage"
)

// SlidingWindowLimiter implements the sliding window rate limiting algorithm
type SlidingWindowLimiter struct {
	store storage.Store
}

// NewSlidingWindowLimiter creates a new sliding window limiter
func NewSlidingWindowLimiter(store storage.Store) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		store: store,
	}
}

// Check implements the Limiter interface for sliding window
func (s *SlidingWindowLimiter) Check(ctx context.Context, key string, limit int, window time.Duration) (limiter.CheckResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	storageKey := "sw:" + key

	// Prune old timestamps
	err := s.store.PruneTimestamps(ctx, storageKey, windowStart)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("sliding_window", "failed to prune timestamps", err)
	}

	// Get valid timestamps within the window
	timestamps, err := s.store.GetTimestamps(ctx, storageKey, windowStart)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("sliding_window", "failed to get timestamps", err)
	}

	currentCount := len(timestamps)

	// Check if limit is exceeded
	if currentCount >= limit {
		var retryAfter int
		if len(timestamps) > 0 {
			oldest := timestamps[0]
			resetAt := oldest.Add(window)
			retryAfter = int(resetAt.Sub(now).Seconds())
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

		return limiter.CheckResult{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			ResetAt:    now.Add(window),
			RetryAfter: int(window.Seconds()),
		}, nil
	}

	// Add the current timestamp
	err = s.store.AddTimestamp(ctx, storageKey, now)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("sliding_window", "failed to add timestamp", err)
	}

	remaining := limit - currentCount - 1
	resetAt := now.Add(window)

	return limiter.CheckResult{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}