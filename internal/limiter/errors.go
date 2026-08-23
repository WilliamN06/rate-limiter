package limiter

import "fmt"

// AlgorithmError represents errors specific to rate limiting algorithms
type AlgorithmError struct {
	Algorithm string
	Message   string
	Err       error
}

func (e *AlgorithmError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("algorithm %s: %s: %v", e.Algorithm, e.Message, e.Err)
	}
	return fmt.Sprintf("algorithm %s: %s", e.Algorithm, e.Message)
}

func (e *AlgorithmError) Unwrap() error {
	return e.Err
}

func NewAlgorithmError(algorithm, message string, err error) *AlgorithmError {
	return &AlgorithmError{
		Algorithm: algorithm,
		Message:   message,
		Err:       err,
	}
}

var (
	ErrInvalidLimit      = fmt.Errorf("limit must be greater than 0")
	ErrInvalidWindow     = fmt.Errorf("window must be greater than 0")
	ErrInvalidCapacity   = fmt.Errorf("capacity must be greater than 0")
	ErrInvalidRefillRate = fmt.Errorf("refill rate must be greater than 0")
	ErrKeyNotFound       = fmt.Errorf("key not found in storage")
)