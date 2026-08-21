package storage

import (
	"context"
	"time"
)

// TokenState represents the state of a token bucket
type TokenState struct {
	Tokens     int       // Current number of tokens
	LastRefill time.Time // Time of last refill
}

// StoreStats contains statistics about the store
type StoreStats struct {
	// Counters is the number of counter keys
	Counters int
	
	// TimestampKeys is the number of keys with timestamps
	TimestampKeys int
	
	// TimestampEntries is the total number of timestamp entries
	TimestampEntries int
	
	// TokenKeys is the number of token bucket keys
	TokenKeys int
	
	// MemoryUsageBytes is an estimate of memory usage
	MemoryUsageBytes int64
}

// CounterStore handles counter-based operations (for fixed window)
type CounterStore interface {
	// Increment atomically increments the counter for a key and returns the new value
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	
	// Get returns the current counter value for a key
	Get(ctx context.Context, key string) (int64, error)
	
	// GetCounters returns all counters for persistence
	GetCounters(ctx context.Context) (map[string]int64, error)
}

// TimestampStore handles timestamp-based operations (for sliding window)
type TimestampStore interface {
	// AddTimestamp adds a timestamp for a key
	AddTimestamp(ctx context.Context, key string, ts time.Time) error
	
	// GetTimestamps returns timestamps for a key that are after the given time
	GetTimestamps(ctx context.Context, key string, after time.Time) ([]time.Time, error)
	
	// PruneTimestamps removes timestamps older than the given time
	PruneTimestamps(ctx context.Context, key string, before time.Time) error
}

// TokenStore handles token bucket state
type TokenStore interface {
	// GetTokenState returns the current token state for a key
	GetTokenState(ctx context.Context, key string) (*TokenState, error)
	
	// UpdateTokenState updates the token state for a key
	UpdateTokenState(ctx context.Context, key string, state *TokenState) error
}

// Store combines all storage interfaces and adds lifecycle methods
type Store interface {
	CounterStore
	TimestampStore
	TokenStore
	
	// Close cleans up any resources
	Close(ctx context.Context) error
	
	// Flush forces any pending writes to persistent storage
	Flush(ctx context.Context) error
	
	// Load loads data from persistent storage
	Load(ctx context.Context) error
	
	// Stats returns store statistics
	Stats(ctx context.Context) (StoreStats, error)
}