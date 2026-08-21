package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WilliamN06/rate-limiter/internal/storage"
	"github.com/WilliamN06/rate-limiter/internal/storage/memory"
)

func TestInMemoryStore_Increment(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	t.Run("increment new key", func(t *testing.T) {
		val, err := store.Increment(ctx, "test:key1", 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), val)
	})

	t.Run("increment existing key", func(t *testing.T) {
		_, err := store.Increment(ctx, "test:key1", 1)
		require.NoError(t, err)
		
		val, err := store.Increment(ctx, "test:key1", 5)
		require.NoError(t, err)
		assert.Equal(t, int64(7), val)
	})

	t.Run("increment with delta 0", func(t *testing.T) {
		val, err := store.Increment(ctx, "test:key2", 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), val)
	})

	t.Run("increment with negative delta", func(t *testing.T) {
		store.Clear()
		
		_, err := store.Increment(ctx, "test:neg", 10)
		require.NoError(t, err)
		
		val, err := store.Increment(ctx, "test:neg", -3)
		require.NoError(t, err)
		assert.Equal(t, int64(7), val)
	})
}

func TestInMemoryStore_Get(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	t.Run("get existing key", func(t *testing.T) {
		_, err := store.Increment(ctx, "test:get1", 5)
		require.NoError(t, err)
		
		val, err := store.Get(ctx, "test:get1")
		require.NoError(t, err)
		assert.Equal(t, int64(5), val)
	})

	t.Run("get non-existing key", func(t *testing.T) {
		val, err := store.Get(ctx, "test:doesnotexist")
		require.NoError(t, err)
		assert.Equal(t, int64(0), val)
	})
}

func TestInMemoryStore_GetCounters(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()

	t.Run("get multiple counters", func(t *testing.T) {
		store.Clear()
		
		_, err := store.Increment(ctx, "key1", 1)
		require.NoError(t, err)
		_, err = store.Increment(ctx, "key2", 2)
		require.NoError(t, err)
		_, err = store.Increment(ctx, "key3", 3)
		require.NoError(t, err)
		
		counters, err := store.GetCounters(ctx)
		require.NoError(t, err)
		
		assert.Equal(t, int64(1), counters["key1"])
		assert.Equal(t, int64(2), counters["key2"])
		assert.Equal(t, int64(3), counters["key3"])
		assert.Len(t, counters, 3)
	})

	t.Run("get counters when empty", func(t *testing.T) {
		store.Clear()
		
		counters, err := store.GetCounters(ctx)
		require.NoError(t, err)
		assert.Empty(t, counters)
	})
}

func TestInMemoryStore_RingBuffer(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	key := "test:timestamps"

	t.Run("add and get timestamps", func(t *testing.T) {
		store.Clear()
		
		now := time.Now()
		
		err := store.AddTimestamp(ctx, key, now)
		require.NoError(t, err)
		
		err = store.AddTimestamp(ctx, key, now.Add(1*time.Second))
		require.NoError(t, err)
		
		timestamps, err := store.GetTimestamps(ctx, key, now.Add(-1*time.Second))
		require.NoError(t, err)
		assert.Len(t, timestamps, 2)
	})

	t.Run("get timestamps after a certain time", func(t *testing.T) {
		store.Clear()
		
		now := time.Now()
		
		err := store.AddTimestamp(ctx, key, now)
		require.NoError(t, err)
		err = store.AddTimestamp(ctx, key, now.Add(2*time.Second))
		require.NoError(t, err)
		
		timestamps, err := store.GetTimestamps(ctx, key, now.Add(1*time.Second))
		require.NoError(t, err)
		assert.Len(t, timestamps, 1)
		assert.True(t, timestamps[0].After(now.Add(1*time.Second)))
	})

	t.Run("prune old timestamps", func(t *testing.T) {
		store.Clear()
		
		now := time.Now()
		
		err := store.AddTimestamp(ctx, key, now.Add(-10*time.Minute))
		require.NoError(t, err)
		err = store.AddTimestamp(ctx, key, now.Add(-5*time.Minute))
		require.NoError(t, err)
		err = store.AddTimestamp(ctx, key, now)
		require.NoError(t, err)
		
		err = store.PruneTimestamps(ctx, key, now.Add(-6*time.Minute))
		require.NoError(t, err)
		
		timestamps, err := store.GetTimestamps(ctx, key, now.Add(-10*time.Minute))
		require.NoError(t, err)
		assert.Len(t, timestamps, 2)
		assert.True(t, timestamps[0].After(now.Add(-6*time.Minute)))
		assert.True(t, timestamps[1].After(now.Add(-6*time.Minute)))
	})

	t.Run("ring buffer capacity", func(t *testing.T) {
		store.Clear()
		
		for i := 0; i < 1100; i++ {
			err := store.AddTimestamp(ctx, key, time.Now())
			require.NoError(t, err)
		}
		
		timestamps, err := store.GetTimestamps(ctx, key, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.LessOrEqual(t, len(timestamps), 1000)
	})
}

func TestInMemoryStore_TokenBucket(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	key := "test:tokens"

	t.Run("get non-existing token state", func(t *testing.T) {
		state, err := store.GetTokenState(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run("update and get token state", func(t *testing.T) {
		store.Clear()
		
		now := time.Now()
		state := &storage.TokenState{
			Tokens:     10,
			LastRefill: now,
		}
		
		err := store.UpdateTokenState(ctx, key, state)
		require.NoError(t, err)
		
		retrieved, err := store.GetTokenState(ctx, key)
		require.NoError(t, err)
		
		assert.NotNil(t, retrieved)
		assert.Equal(t, 10, retrieved.Tokens)
		assert.Equal(t, now.Unix(), retrieved.LastRefill.Unix())
	})

	t.Run("update existing token state", func(t *testing.T) {
		store.Clear()
		
		now := time.Now()
		state := &storage.TokenState{
			Tokens:     10,
			LastRefill: now,
		}
		
		err := store.UpdateTokenState(ctx, key, state)
		require.NoError(t, err)
		
		newState := &storage.TokenState{
			Tokens:     5,
			LastRefill: now.Add(1 * time.Second),
		}
		
		err = store.UpdateTokenState(ctx, key, newState)
		require.NoError(t, err)
		
		retrieved, err := store.GetTokenState(ctx, key)
		require.NoError(t, err)
		
		assert.Equal(t, 5, retrieved.Tokens)
		assert.Equal(t, now.Add(1*time.Second).Unix(), retrieved.LastRefill.Unix())
	})

	t.Run("multiple token keys", func(t *testing.T) {
		store.Clear()
		
		keys := []string{"key1", "key2", "key3"}
		now := time.Now()
		
		for i, key := range keys {
			state := &storage.TokenState{
				Tokens:     i * 10,
				LastRefill: now,
			}
			err := store.UpdateTokenState(ctx, key, state)
			require.NoError(t, err)
		}
		
		for i, key := range keys {
			state, err := store.GetTokenState(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, i*10, state.Tokens)
		}
	})
}

func TestInMemoryStore_Concurrency(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	t.Run("concurrent increments", func(t *testing.T) {
		store.Clear()
		
		var wg sync.WaitGroup
		key := "test:concurrent"
		numRoutines := 100
		incrementsPerRoutine := 10
		
		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < incrementsPerRoutine; j++ {
					_, err := store.Increment(ctx, key, 1)
					assert.NoError(t, err)
				}
			}()
		}
		
		wg.Wait()
		
		val, err := store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(numRoutines*incrementsPerRoutine), val)
	})

	t.Run("concurrent timestamp additions", func(t *testing.T) {
		store.Clear()
		
		var wg sync.WaitGroup
		key := "test:concurrent-ts"
		numRoutines := 50
		addsPerRoutine := 20
		
		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < addsPerRoutine; j++ {
					err := store.AddTimestamp(ctx, key, time.Now())
					assert.NoError(t, err)
				}
			}()
		}
		
		wg.Wait()
		
		timestamps, err := store.GetTimestamps(ctx, key, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, numRoutines*addsPerRoutine, len(timestamps))
	})

	t.Run("mixed operations", func(t *testing.T) {
		store.Clear()
		
		var wg sync.WaitGroup
		
		// Counter operations - 50 increments
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				key := "test:mixed"
				_, err := store.Increment(ctx, key, 1)
				assert.NoError(t, err)
			}(i)
		}
		
		// Timestamp operations - 50 additions
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				key := "test:mixed-ts"
				err := store.AddTimestamp(ctx, key, time.Now())
				assert.NoError(t, err)
			}()
		}
		
		// Token operations - 50 unique keys
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Use unique key for each goroutine
				key := "test:mixed-token:" + string(rune('a'+idx%26)) + string(rune('a'+idx/26))
				state := &storage.TokenState{
					Tokens:     idx,
					LastRefill: time.Now(),
				}
				err := store.UpdateTokenState(ctx, key, state)
				assert.NoError(t, err)
			}(i)
		}
		
		wg.Wait()
		
		// Verify counters
		val, err := store.Get(ctx, "test:mixed")
		require.NoError(t, err)
		assert.Equal(t, int64(50), val)
		
		// Verify timestamps
		timestamps, err := store.GetTimestamps(ctx, "test:mixed-ts", time.Now().Add(-1*time.Second))
		require.NoError(t, err)
		assert.Equal(t, 50, len(timestamps))
		
		// Verify tokens - should have 50 unique keys
		stats, err := store.Stats(ctx)
		require.NoError(t, err)
		assert.Equal(t, 50, stats.TokenKeys)
	})
}

func TestInMemoryStore_Stats(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	t.Run("stats after operations", func(t *testing.T) {
		store.Clear()
		
		for i := 0; i < 10; i++ {
			key := "counter:" + string(rune('a'+i))
			_, err := store.Increment(ctx, key, 1)
			require.NoError(t, err)
		}
		
		for i := 0; i < 5; i++ {
			key := "timestamp:" + string(rune('a'+i))
			for j := 0; j < 3; j++ {
				err := store.AddTimestamp(ctx, key, time.Now())
				require.NoError(t, err)
			}
		}
		
		for i := 0; i < 7; i++ {
			key := "token:" + string(rune('a'+i))
			state := &storage.TokenState{
				Tokens:     10,
				LastRefill: time.Now(),
			}
			err := store.UpdateTokenState(ctx, key, state)
			require.NoError(t, err)
		}
		
		stats, err := store.Stats(ctx)
		require.NoError(t, err)
		
		assert.Equal(t, 10, stats.Counters)
		assert.Equal(t, 5, stats.TimestampKeys)
		assert.Equal(t, 15, stats.TimestampEntries)
		assert.Equal(t, 7, stats.TokenKeys)
		assert.Greater(t, stats.MemoryUsageBytes, int64(0))
	})

	t.Run("stats when empty", func(t *testing.T) {
		store.Clear()
		
		stats, err := store.Stats(ctx)
		require.NoError(t, err)
		
		assert.Equal(t, 0, stats.Counters)
		assert.Equal(t, 0, stats.TimestampKeys)
		assert.Equal(t, 0, stats.TimestampEntries)
		assert.Equal(t, 0, stats.TokenKeys)
		assert.Equal(t, int64(0), stats.MemoryUsageBytes)
	})
}

func TestInMemoryStore_MaxKeys(t *testing.T) {
	config := &memory.StoreConfig{
		MaxKeys:            5,
		RingBufferCapacity: 10,
		CleanupInterval:    0,
	}
	
	store := memory.NewInMemoryStoreWithConfig(config)
	ctx := context.Background()
	
	t.Run("counter max keys", func(t *testing.T) {
		store.Clear()
		
		for i := 0; i < 5; i++ {
			key := "counter:" + string(rune('a'+i))
			_, err := store.Increment(ctx, key, 1)
			require.NoError(t, err)
		}
		
		_, err := store.Increment(ctx, "counter:overflow", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum number of keys")
	})

	t.Run("timestamp max keys", func(t *testing.T) {
		store.Clear()
		
		for i := 0; i < 5; i++ {
			key := "timestamp:" + string(rune('a'+i))
			err := store.AddTimestamp(ctx, key, time.Now())
			require.NoError(t, err)
		}
		
		err := store.AddTimestamp(ctx, "timestamp:overflow", time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum number of timestamp keys")
	})

	t.Run("token max keys", func(t *testing.T) {
		store.Clear()
		
		for i := 0; i < 5; i++ {
			key := "token:" + string(rune('a'+i))
			state := &storage.TokenState{Tokens: 10, LastRefill: time.Now()}
			err := store.UpdateTokenState(ctx, key, state)
			require.NoError(t, err)
		}
		
		state := &storage.TokenState{Tokens: 10, LastRefill: time.Now()}
		err := store.UpdateTokenState(ctx, "token:overflow", state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum number of token keys")
	})
}

func TestInMemoryStore_Cleanup(t *testing.T) {
	config := &memory.StoreConfig{
		MaxKeys:            0,
		RingBufferCapacity: 100,
		CleanupInterval:    100 * time.Millisecond,
	}
	
	store := memory.NewInMemoryStoreWithConfig(config)
	ctx := context.Background()
	
	t.Run("automatic cleanup of old timestamps", func(t *testing.T) {
		store.Clear()
		
		key := "test:cleanup"
		
		oldTime := time.Now().Add(-2 * time.Hour)
		for i := 0; i < 50; i++ {
			err := store.AddTimestamp(ctx, key, oldTime.Add(time.Duration(i)*time.Second))
			require.NoError(t, err)
		}
		
		time.Sleep(200 * time.Millisecond)
		
		timestamps, err := store.GetTimestamps(ctx, key, time.Now().Add(-3*time.Hour))
		require.NoError(t, err)
		
		assert.Less(t, len(timestamps), 50)
	})
}

// Benchmark tests
func BenchmarkInMemoryStore_Increment(b *testing.B) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Increment(ctx, "bench:key", 1)
	}
}

func BenchmarkInMemoryStore_Get(b *testing.B) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	_, _ = store.Increment(ctx, "bench:key", 100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, "bench:key")
	}
}

func BenchmarkInMemoryStore_AddTimestamp(b *testing.B) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.AddTimestamp(ctx, "bench:key", time.Now())
	}
}

func BenchmarkInMemoryStore_GetTimestamps(b *testing.B) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	for i := 0; i < 100; i++ {
		_ = store.AddTimestamp(ctx, "bench:key", time.Now())
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetTimestamps(ctx, "bench:key", time.Now().Add(-1*time.Hour))
	}
}

func BenchmarkInMemoryStore_UpdateTokenState(b *testing.B) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	
	state := &storage.TokenState{
		Tokens:     10,
		LastRefill: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.UpdateTokenState(ctx, "bench:key", state)
	}
}