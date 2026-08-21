package limiter

import "time"

// Algorithm represents the rate limiting strategy
type Algorithm string

const (
	// FixedWindow divides time into fixed windows and counts requests per window
	FixedWindow Algorithm = "fixed_window"
	
	// SlidingWindow uses a sliding window based on request timestamps
	SlidingWindow Algorithm = "sliding_window"
	
	// TokenBucket allows bursts up to capacity, refilling at a fixed rate
	TokenBucket Algorithm = "token_bucket"
)

// CheckResult contains the result of a rate limit check
type CheckResult struct {
	// Allowed indicates whether the request is allowed
	Allowed bool
	
	// Limit is the maximum number of requests allowed in the window
	Limit int
	
	// Remaining is the number of requests still allowed in the current window
	Remaining int
	
	// ResetAt is the time when the current window resets
	ResetAt time.Time
	
	// RetryAfter is the number of seconds to wait before retrying (only set when Allowed is false)
	RetryAfter int
}

// AlgorithmConfig contains configuration for a specific algorithm
type AlgorithmConfig struct {
	// Type is the algorithm to use
	Type Algorithm
	
	// Window is the time window for rate limiting
	Window time.Duration
	
	// Limit is the maximum number of requests allowed
	Limit int
	
	// TokenBucket specific configuration
	Capacity    int     // Maximum tokens in the bucket
	RefillRate  float64 // Tokens added per second
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