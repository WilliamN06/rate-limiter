package limiter_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/limiter/algorithms"
	"github.com/WilliamN06/rate-limiter/internal/storage/memory"
)

// newTestEngine creates an engine with all algorithms registered
func newTestEngine(t *testing.T) *limiter.Engine {
	store := memory.NewInMemoryStore()

	algorithmsMap := make(map[limiter.Algorithm]limiter.Limiter)
	algorithmsMap[limiter.FixedWindow] = algorithms.NewFixedWindowLimiter(store)
	algorithmsMap[limiter.SlidingWindow] = algorithms.NewSlidingWindowLimiter(store)
	algorithmsMap[limiter.TokenBucket] = algorithms.NewTokenBucketLimiter(store)

	engine, err := limiter.NewEngine(&limiter.EngineConfig{
		Store:            store,
		DefaultAlgorithm: limiter.FixedWindow,
		Algorithms:       algorithmsMap,
	})
	require.NoError(t, err)
	return engine
}

func TestEngine_FixedWindow(t *testing.T) {
	ctx := context.Background()
	key := "test:fixed"
	limit := 5
	window := 2 * time.Second

	t.Run("allows requests up to limit", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, limit-i-1, result.Remaining)
		}
	})

	t.Run("rate limits after limit exceeded", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit+5; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)

			if i < limit {
				assert.True(t, result.Allowed)
			} else {
				assert.False(t, result.Allowed)
				assert.Equal(t, 0, result.Remaining)
				assert.Greater(t, result.RetryAfter, 0)
			}
		}
	})

	t.Run("resets after window expires", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		}

		// Next request should be rate limited
		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// Wait for the window to fully expire (add buffer)
		time.Sleep(window + 1*time.Second)

		// Should be allowed again
		result, err = engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	t.Run("different keys have independent limits", func(t *testing.T) {
		engine := newTestEngine(t)

		key1 := "test:fixed:key1"
		key2 := "test:fixed:key2"

		for i := 0; i < limit; i++ {
			_, err := engine.Check(ctx, key1, limit, window)
			require.NoError(t, err)
		}

		result, err := engine.Check(ctx, key1, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		result, err = engine.Check(ctx, key2, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, limit-1, result.Remaining)
	})
}

func TestEngine_SlidingWindow(t *testing.T) {
	ctx := context.Background()
	key := "test:sliding"
	limit := 5
	window := 2 * time.Second

	t.Run("allows requests up to limit", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, limit-i-1, result.Remaining)
		}

		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
	})

	t.Run("sliding window allows requests after old timestamps expire", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		}

		// Should be rate limited
		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// Wait for full window to expire
		time.Sleep(window + 1*time.Second)

		// Should be allowed again
		result, err = engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})
}

func TestEngine_TokenBucket(t *testing.T) {
	ctx := context.Background()
	key := "test:token"
	limit := 5
	window := 2 * time.Second

	t.Run("allows bursts up to capacity", func(t *testing.T) {
		engine := newTestEngine(t)

		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, limit-i-1, result.Remaining)
		}

		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		engine := newTestEngine(t)

		// Consume all tokens
		for i := 0; i < limit; i++ {
			result, err := engine.Check(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		}

		// Should be rate limited
		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// Wait for full refill (window duration should refill all tokens)
		time.Sleep(window + 1*time.Second)

		// Should be allowed again with full tokens
		result, err = engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		// Should have limit-1 remaining after consuming one token
		assert.Equal(t, limit-1, result.Remaining)
	})
}

func TestEngine_Concurrency(t *testing.T) {
	ctx := context.Background()
	key := "test:concurrent"
	limit := 100
	window := 10 * time.Second

	t.Run("concurrent requests with fixed window", func(t *testing.T) {
		engine := newTestEngine(t)

		var wg sync.WaitGroup
		numRequests := 150
		allowedCount := 0
		var mu sync.Mutex

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := engine.Check(ctx, key, limit, window)
				require.NoError(t, err)

				mu.Lock()
				if result.Allowed {
					allowedCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		assert.LessOrEqual(t, allowedCount, limit)
		assert.Greater(t, allowedCount, 0)
	})
}

func TestEngine_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("limit of 1", func(t *testing.T) {
		engine := newTestEngine(t)
		key := "test:limit1"
		limit := 1
		window := 1 * time.Second

		result, err := engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, 0, result.Remaining)

		result, err = engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// Wait for window to expire
		time.Sleep(window + 1*time.Second)

		result, err = engine.Check(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	t.Run("empty key returns error", func(t *testing.T) {
		engine := newTestEngine(t)
		_, err := engine.Check(ctx, "", 10, 10*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})

	t.Run("zero limit returns error", func(t *testing.T) {
		engine := newTestEngine(t)
		_, err := engine.Check(ctx, "test:zero", 0, 10*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "limit must be greater than 0")
	})

	t.Run("negative limit returns error", func(t *testing.T) {
		engine := newTestEngine(t)
		_, err := engine.Check(ctx, "test:negative", -1, 10*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "limit must be greater than 0")
	})
}

func TestEngine_GetSupportedAlgorithms(t *testing.T) {
	engine := newTestEngine(t)

	algorithms := engine.GetSupportedAlgorithms()
	assert.Contains(t, algorithms, limiter.FixedWindow)
	assert.Contains(t, algorithms, limiter.SlidingWindow)
	assert.Contains(t, algorithms, limiter.TokenBucket)
	assert.Len(t, algorithms, 3)
}

func TestEngine_DefaultAlgorithm(t *testing.T) {
	engine := newTestEngine(t)

	assert.Equal(t, limiter.FixedWindow, engine.GetDefaultAlgorithm())

	err := engine.SetDefaultAlgorithm(limiter.TokenBucket)
	require.NoError(t, err)
	assert.Equal(t, limiter.TokenBucket, engine.GetDefaultAlgorithm())

	err = engine.SetDefaultAlgorithm("invalid")
	assert.Error(t, err)
}

func BenchmarkEngine_FixedWindow(b *testing.B) {
	store := memory.NewInMemoryStore()
	algorithmsMap := make(map[limiter.Algorithm]limiter.Limiter)
	algorithmsMap[limiter.FixedWindow] = algorithms.NewFixedWindowLimiter(store)
	algorithmsMap[limiter.SlidingWindow] = algorithms.NewSlidingWindowLimiter(store)
	algorithmsMap[limiter.TokenBucket] = algorithms.NewTokenBucketLimiter(store)

	engine, _ := limiter.NewEngine(&limiter.EngineConfig{
		Store:            store,
		DefaultAlgorithm: limiter.FixedWindow,
		Algorithms:       algorithmsMap,
	})

	ctx := context.Background()
	key := "bench:fixed"
	limit := 100
	window := 10 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Check(ctx, key, limit, window)
	}
}

func BenchmarkEngine_SlidingWindow(b *testing.B) {
	store := memory.NewInMemoryStore()
	algorithmsMap := make(map[limiter.Algorithm]limiter.Limiter)
	algorithmsMap[limiter.FixedWindow] = algorithms.NewFixedWindowLimiter(store)
	algorithmsMap[limiter.SlidingWindow] = algorithms.NewSlidingWindowLimiter(store)
	algorithmsMap[limiter.TokenBucket] = algorithms.NewTokenBucketLimiter(store)

	engine, _ := limiter.NewEngine(&limiter.EngineConfig{
		Store:            store,
		DefaultAlgorithm: limiter.FixedWindow,
		Algorithms:       algorithmsMap,
	})

	ctx := context.Background()
	key := "bench:sliding"
	limit := 100
	window := 10 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Check(ctx, key, limit, window)
	}
}

func BenchmarkEngine_TokenBucket(b *testing.B) {
	store := memory.NewInMemoryStore()
	algorithmsMap := make(map[limiter.Algorithm]limiter.Limiter)
	algorithmsMap[limiter.FixedWindow] = algorithms.NewFixedWindowLimiter(store)
	algorithmsMap[limiter.SlidingWindow] = algorithms.NewSlidingWindowLimiter(store)
	algorithmsMap[limiter.TokenBucket] = algorithms.NewTokenBucketLimiter(store)

	engine, _ := limiter.NewEngine(&limiter.EngineConfig{
		Store:            store,
		DefaultAlgorithm: limiter.FixedWindow,
		Algorithms:       algorithmsMap,
	})

	ctx := context.Background()
	key := "bench:token"
	limit := 100
	window := 10 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Check(ctx, key, limit, window)
	}
}