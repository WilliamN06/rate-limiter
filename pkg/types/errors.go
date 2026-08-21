package types

import "fmt"

// ErrorCode represents a specific error type
type ErrorCode string

const (
	// Common errors
	ErrInvalidRequest   ErrorCode = "INVALID_REQUEST"
	ErrInternalError    ErrorCode = "INTERNAL_ERROR"
	ErrStorageError     ErrorCode = "STORAGE_ERROR"
	
	// Rate limiting specific
	ErrRateLimited      ErrorCode = "RATE_LIMITED"
	ErrAlgorithmInvalid ErrorCode = "ALGORITHM_INVALID"
)

// RateLimiterError represents a rate limiter error
type RateLimiterError struct {
	Code    ErrorCode
	Message string
	Details string
	Err     error
}

func (e *RateLimiterError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the errors.Wrapper interface
func (e *RateLimiterError) Unwrap() error {
	return e.Err
}

// NewError creates a new RateLimiterError
func NewError(code ErrorCode, message string, details string, err error) *RateLimiterError {
	return &RateLimiterError{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

// IsRateLimited checks if an error is a rate limit error
func IsRateLimited(err error) bool {
	if rle, ok := err.(*RateLimiterError); ok {
		return rle.Code == ErrRateLimited
	}
	return false
}