package algorithms

import (
	"context"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/storage"
)

// TokenBucketLimiter implements the token bucket rate limiting algorithm
type TokenBucketLimiter struct {
	store storage.Store
}

// NewTokenBucketLimiter creates a new token bucket limiter
func NewTokenBucketLimiter(store storage.Store) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		store: store,
	}
}

// Check implements the Limiter interface for token bucket
func (t *TokenBucketLimiter) Check(ctx context.Context, key string, limit int, window time.Duration) (limiter.CheckResult, error) {
	now := time.Now()
	storageKey := "tb:" + key

	// Calculate refill rate (tokens per second)
	refillRate := float64(limit) / window.Seconds()
	capacity := limit

	// Get existing token state
	state, err := t.store.GetTokenState(ctx, storageKey)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("token_bucket", "failed to get token state", err)
	}

	// Initialize state if it doesn't exist
	if state == nil {
		state = &storage.TokenState{
			Tokens:     capacity,
			LastRefill: now,
		}
	} else {
		// Calculate tokens to add since last refill
		elapsed := now.Sub(state.LastRefill)
		tokensToAdd := int(float64(elapsed.Nanoseconds()) / 1e9 * refillRate)

		if tokensToAdd > 0 {
			newTokens := state.Tokens + tokensToAdd
			if newTokens > capacity {
				state.Tokens = capacity
			} else {
				state.Tokens = newTokens
			}
			state.LastRefill = now
		}
	}

	// Check if we have tokens available
	if state.Tokens <= 0 {
		// Calculate time until next token
		timeToNextToken := time.Duration(1.0/refillRate) * time.Second
		if timeToNextToken < time.Millisecond {
			timeToNextToken = time.Millisecond
		}
		resetAt := now.Add(timeToNextToken)
		retryAfter := int(timeToNextToken.Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}

		// Save state
		err = t.store.UpdateTokenState(ctx, storageKey, state)
		if err != nil {
			return limiter.CheckResult{}, limiter.NewAlgorithmError("token_bucket", "failed to update token state", err)
		}

		return limiter.CheckResult{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: retryAfter,
		}, nil
	}

	// Consume one token
	state.Tokens--

	// Save state
	err = t.store.UpdateTokenState(ctx, storageKey, state)
	if err != nil {
		return limiter.CheckResult{}, limiter.NewAlgorithmError("token_bucket", "failed to update token state", err)
	}

	// Calculate when bucket will be full
	timeToFull := time.Duration(float64(capacity-state.Tokens)/refillRate) * time.Second
	resetAt := now.Add(timeToFull)

	return limiter.CheckResult{
		Allowed:   true,
		Limit:     limit,
		Remaining: state.Tokens,
		ResetAt:   resetAt,
	}, nil
}