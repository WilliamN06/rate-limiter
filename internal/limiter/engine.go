package limiter

import (
	"context"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/storage"
)

// Engine is the main rate limiter engine
type Engine struct {
	store       storage.Store
	algorithms  map[Algorithm]Limiter
	defaultAlgo Algorithm
}

// EngineConfig configures the rate limiter engine
type EngineConfig struct {
	Store            storage.Store
	DefaultAlgorithm Algorithm
	Algorithms       map[Algorithm]Limiter
}

// NewEngine creates a new rate limiter engine
func NewEngine(config *EngineConfig) (*Engine, error) {
	if config == nil {
		return nil, NewAlgorithmError("engine", "config is required", nil)
	}

	if config.Store == nil {
		return nil, NewAlgorithmError("engine", "store is required", nil)
	}

	defaultAlgo := config.DefaultAlgorithm
	if defaultAlgo == "" {
		defaultAlgo = FixedWindow
	}

	algorithms := make(map[Algorithm]Limiter)
	for name, impl := range config.Algorithms {
		algorithms[name] = impl
	}

	return &Engine{
		store:       config.Store,
		algorithms:  algorithms,
		defaultAlgo: defaultAlgo,
	}, nil
}

// Check uses the default algorithm
func (e *Engine) Check(ctx context.Context, key string, limit int, window time.Duration) (CheckResult, error) {
	return e.CheckWithAlgorithm(ctx, key, limit, window, e.defaultAlgo)
}

// CheckWithAlgorithm uses a specific algorithm
func (e *Engine) CheckWithAlgorithm(ctx context.Context, key string, limit int, window time.Duration, algo Algorithm) (CheckResult, error) {
	if key == "" {
		return CheckResult{}, NewAlgorithmError(string(algo), "key cannot be empty", nil)
	}

	if limit <= 0 {
		return CheckResult{}, NewAlgorithmError(string(algo), "limit must be greater than 0", ErrInvalidLimit)
	}

	if window <= 0 {
		return CheckResult{}, NewAlgorithmError(string(algo), "window must be greater than 0", ErrInvalidWindow)
	}

	limiter, ok := e.algorithms[algo]
	if !ok {
		return CheckResult{}, NewAlgorithmError(string(algo), "algorithm not supported", nil)
	}

	return limiter.Check(ctx, key, limit, window)
}

// GetSupportedAlgorithms returns all supported algorithms
func (e *Engine) GetSupportedAlgorithms() []Algorithm {
	algorithms := make([]Algorithm, 0, len(e.algorithms))
	for name := range e.algorithms {
		algorithms = append(algorithms, name)
	}
	return algorithms
}

// GetDefaultAlgorithm returns the default algorithm
func (e *Engine) GetDefaultAlgorithm() Algorithm {
	return e.defaultAlgo
}

// SetDefaultAlgorithm sets the default algorithm
func (e *Engine) SetDefaultAlgorithm(algo Algorithm) error {
	if _, ok := e.algorithms[algo]; !ok {
		return NewAlgorithmError(string(algo), "algorithm not supported", nil)
	}
	e.defaultAlgo = algo
	return nil
}