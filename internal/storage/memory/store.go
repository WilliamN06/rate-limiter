package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/storage"
)

const (
	// DefaultRingBufferCapacity is the default max timestamps per key
	DefaultRingBufferCapacity = 1000
	
	// DefaultCleanupInterval is how often to clean up stale entries
	DefaultCleanupInterval = 5 * time.Minute
	
	// DefaultMaxKeys is the maximum number of keys to track
	DefaultMaxKeys = 10000
)

// InMemoryStore implements storage.Store using in-memory data structures
type InMemoryStore struct {
	// Counters for fixed window algorithm
	counters   sync.Map // key -> *atomic.Int64
	
	// Timestamp buffers for sliding window algorithm
	timestamps sync.Map // key -> *RingBuffer
	
	// Token bucket states
	tokens     sync.Map // key -> *storage.TokenState
	
	// Configuration
	maxKeys       int
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	
	// Statistics
	stats struct {
		sync.RWMutex
		counterKeys     int64
		timestampKeys   int64
		tokenKeys       int64
		timestampCount  int64 // Total timestamp entries across all keys
	}
}

// Config configures the in-memory store
type StoreConfig struct {
	// MaxKeys is the maximum number of keys to track (0 = unlimited)
	MaxKeys int
	
	// RingBufferCapacity is the max timestamps per key for sliding window
	RingBufferCapacity int
	
	// CleanupInterval is how often to prune old entries (0 = disabled)
	CleanupInterval time.Duration
}

// DefaultConfig returns a reasonable default configuration
func DefaultConfig() *StoreConfig {
	return &StoreConfig{
		MaxKeys:            DefaultMaxKeys,
		RingBufferCapacity: DefaultRingBufferCapacity,
		CleanupInterval:    DefaultCleanupInterval,
	}
}

// NewInMemoryStore creates a new in-memory store with default configuration
func NewInMemoryStore() *InMemoryStore {
	return NewInMemoryStoreWithConfig(DefaultConfig())
}

// NewInMemoryStoreWithConfig creates a new in-memory store with custom configuration
func NewInMemoryStoreWithConfig(config *StoreConfig) *InMemoryStore {
	store := &InMemoryStore{
		maxKeys:       config.MaxKeys,
		stopCleanup:   make(chan struct{}),
	}
	
	// Start cleanup goroutine if interval is set
	if config.CleanupInterval > 0 {
		store.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go store.cleanupLoop()
	}
	
	return store
}

// cleanupLoop periodically prunes stale entries
func (s *InMemoryStore) cleanupLoop() {
	for {
		select {
		case <-s.cleanupTicker.C:
			s.cleanup()
		case <-s.stopCleanup:
			return
		}
	}
}

// cleanup removes old timestamps to prevent unbounded growth
func (s *InMemoryStore) cleanup() {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	
	s.timestamps.Range(func(key, value interface{}) bool {
		if rb, ok := value.(*RingBuffer); ok {
			// Prune timestamps older than 1 hour
			removed := rb.Prune(oneHourAgo)
			if removed > 0 {
				atomic.AddInt64(&s.stats.timestampCount, -int64(removed))
			}
		}
		return true
	})
}

// Increment implements CounterStore.Increment
func (s *InMemoryStore) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if s.maxKeys > 0 {
		// Check if adding a new key would exceed max
		_, ok := s.counters.Load(key)
		if !ok {
			// Count current keys
			var count int64
			s.counters.Range(func(_, _ interface{}) bool {
				count++
				return true
			})
			if count >= int64(s.maxKeys) {
				return 0, fmt.Errorf("maximum number of keys (%d) exceeded", s.maxKeys)
			}
		}
	}
	
	var counter *atomic.Int64
	if val, ok := s.counters.Load(key); ok {
		counter = val.(*atomic.Int64)
	} else {
		counter = &atomic.Int64{}
		actual, _ := s.counters.LoadOrStore(key, counter)
		counter = actual.(*atomic.Int64)
		atomic.AddInt64(&s.stats.counterKeys, 1)
	}
	
	newVal := counter.Add(delta)
	return newVal, nil
}

// Get implements CounterStore.Get
func (s *InMemoryStore) Get(ctx context.Context, key string) (int64, error) {
	if val, ok := s.counters.Load(key); ok {
		counter := val.(*atomic.Int64)
		return counter.Load(), nil
	}
	return 0, nil
}

// GetCounters implements CounterStore.GetCounters
func (s *InMemoryStore) GetCounters(ctx context.Context) (map[string]int64, error) {
	result := make(map[string]int64)
	s.counters.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if counter, ok := value.(*atomic.Int64); ok {
				result[k] = counter.Load()
			}
		}
		return true
	})
	return result, nil
}

// AddTimestamp implements TimestampStore.AddTimestamp
func (s *InMemoryStore) AddTimestamp(ctx context.Context, key string, ts time.Time) error {
	// Check key limit
	if s.maxKeys > 0 {
		_, ok := s.timestamps.Load(key)
		if !ok {
			var count int64
			s.timestamps.Range(func(_, _ interface{}) bool {
				count++
				return true
			})
			if count >= int64(s.maxKeys) {
				return fmt.Errorf("maximum number of timestamp keys (%d) exceeded", s.maxKeys)
			}
		}
	}
	
	var buffer *RingBuffer
	if val, ok := s.timestamps.Load(key); ok {
		buffer = val.(*RingBuffer)
	} else {
		buffer = NewRingBuffer(DefaultRingBufferCapacity)
		actual, _ := s.timestamps.LoadOrStore(key, buffer)
		buffer = actual.(*RingBuffer)
		atomic.AddInt64(&s.stats.timestampKeys, 1)
	}
	
	buffer.Add(ts)
	atomic.AddInt64(&s.stats.timestampCount, 1)
	return nil
}

// GetTimestamps implements TimestampStore.GetTimestamps
func (s *InMemoryStore) GetTimestamps(ctx context.Context, key string, after time.Time) ([]time.Time, error) {
	if val, ok := s.timestamps.Load(key); ok {
		buffer := val.(*RingBuffer)
		return buffer.GetValid(after), nil
	}
	return []time.Time{}, nil
}

// PruneTimestamps implements TimestampStore.PruneTimestamps
func (s *InMemoryStore) PruneTimestamps(ctx context.Context, key string, before time.Time) error {
	if val, ok := s.timestamps.Load(key); ok {
		buffer := val.(*RingBuffer)
		removed := buffer.Prune(before)
		if removed > 0 {
			atomic.AddInt64(&s.stats.timestampCount, -int64(removed))
		}
	}
	return nil
}

// GetTokenState implements TokenStore.GetTokenState
func (s *InMemoryStore) GetTokenState(ctx context.Context, key string) (*storage.TokenState, error) {
	if val, ok := s.tokens.Load(key); ok {
		return val.(*storage.TokenState), nil
	}
	return nil, nil
}

// UpdateTokenState implements TokenStore.UpdateTokenState
func (s *InMemoryStore) UpdateTokenState(ctx context.Context, key string, state *storage.TokenState) error {
	if s.maxKeys > 0 {
		_, ok := s.tokens.Load(key)
		if !ok {
			var count int64
			s.tokens.Range(func(_, _ interface{}) bool {
				count++
				return true
			})
			if count >= int64(s.maxKeys) {
				return fmt.Errorf("maximum number of token keys (%d) exceeded", s.maxKeys)
			}
		}
	}
	
	// Use atomic operation to update state
	atomicState := &storage.TokenState{
		Tokens:     state.Tokens,
		LastRefill: state.LastRefill,
	}
	
	actual, loaded := s.tokens.LoadOrStore(key, atomicState)
	if loaded {
		existing := actual.(*storage.TokenState)
		existing.Tokens = state.Tokens
		existing.LastRefill = state.LastRefill
	} else {
		atomic.AddInt64(&s.stats.tokenKeys, 1)
	}
	
	return nil
}

// Close implements Store.Close
func (s *InMemoryStore) Close(ctx context.Context) error {
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}
	close(s.stopCleanup)
	return nil
}

// Flush implements Store.Flush (no-op for in-memory store)
func (s *InMemoryStore) Flush(ctx context.Context) error {
	// In-memory store doesn't need to flush
	return nil
}

// Load implements Store.Load (no-op for in-memory store)
func (s *InMemoryStore) Load(ctx context.Context) error {
	// In-memory store has no persistent data to load
	return nil
}

// Stats implements Store.Stats
func (s *InMemoryStore) Stats(ctx context.Context) (storage.StoreStats, error) {
	var stats storage.StoreStats
	
	stats.Counters = int(atomic.LoadInt64(&s.stats.counterKeys))
	stats.TimestampKeys = int(atomic.LoadInt64(&s.stats.timestampKeys))
	stats.TimestampEntries = int(atomic.LoadInt64(&s.stats.timestampCount))
	stats.TokenKeys = int(atomic.LoadInt64(&s.stats.tokenKeys))
	
	// Estimate memory usage
	// Each counter: ~80 bytes (key overhead + atomic int)
	stats.MemoryUsageBytes = int64(stats.Counters * 80)
	
	// Each timestamp entry: ~32 bytes (timestamp + overhead)
	stats.MemoryUsageBytes += int64(stats.TimestampEntries * 32)
	
	// Each token state: ~64 bytes
	stats.MemoryUsageBytes += int64(stats.TokenKeys * 64)
	
	// Ring buffer overhead: ~100 bytes per key
	stats.MemoryUsageBytes += int64(stats.TimestampKeys * 100)
	
	return stats, nil
}

// Clear clears all data in the store (primarily for testing)
func (s *InMemoryStore) Clear() {
	s.counters = sync.Map{}
	s.timestamps = sync.Map{}
	s.tokens = sync.Map{}
	
	s.stats.Lock()
	defer s.stats.Unlock()
	s.stats.counterKeys = 0
	s.stats.timestampKeys = 0
	s.stats.tokenKeys = 0
	s.stats.timestampCount = 0
}