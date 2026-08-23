package limiter

import (
	"context"
	"time"
)

// Algorithm represents the rate limiting strategy
type Algorithm string

const (
	FixedWindow   Algorithm = "fixed_window"
	SlidingWindow Algorithm = "sliding_window"
	TokenBucket   Algorithm = "token_bucket"
)

// CheckResult contains the result of a rate limit check
type CheckResult struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter int
}

// Limiter defines the interface for rate limiting
type Limiter interface {
	// Check determines if a request is allowed
	Check(ctx context.Context, key string, limit int, window time.Duration) (CheckResult, error)
}

// NewCheckResult returns a successful CheckResult
func NewCheckResult(limit, remaining int, resetAt time.Time) CheckResult {
	return CheckResult{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}
}

// NewRateLimitedResult returns a rate-limited CheckResult
func NewRateLimitedResult(limit int, resetAt time.Time, retryAfter int) CheckResult {
	return CheckResult{
		Allowed:    false,
		Limit:      limit,
		Remaining:  0,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}
}