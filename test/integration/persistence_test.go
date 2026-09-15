package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WilliamN06/rate-limiter/internal/storage/sqlite"
)

func TestSQLitePersistence(t *testing.T) {
	// Use a temporary database for testing
	dbPath := "./test.db"
	defer os.Remove(dbPath)

	config := &sqlite.Config{
		DBPath:         dbPath,
		FlushInterval:  100 * time.Millisecond,
		FlushThreshold: 10,
		MaxOpenConns:   1,
		MaxIdleConns:   1,
	}

	store, err := sqlite.NewSQLiteStore(config)
	require.NoError(t, err)
	defer store.Close(context.Background())

	ctx := context.Background()

	t.Run("increment and get counter", func(t *testing.T) {
		key := "test:counter:1"
		
		val, err := store.Increment(ctx, key, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), val)

		val, err = store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(1), val)
	})

	t.Run("add and get timestamps", func(t *testing.T) {
		key := "test:timestamps:1"
		now := time.Now()

		err := store.AddTimestamp(ctx, key, now)
		require.NoError(t, err)

		err = store.AddTimestamp(ctx, key, now.Add(time.Second))
		require.NoError(t, err)

		timestamps, err := store.GetTimestamps(ctx, key, now.Add(-time.Second))
		require.NoError(t, err)
		assert.Len(t, timestamps, 2)
	})

	t.Run("token state", func(t *testing.T) {
		key := "test:tokens:1"
		now := time.Now()

		state := &sqlite.TokenState{
			Tokens:     10,
			LastRefill: now,
		}

		err := store.UpdateTokenState(ctx, key, state)
		require.NoError(t, err)

		retrieved, err := store.GetTokenState(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 10, retrieved.Tokens)
		assert.Equal(t, now.Unix(), retrieved.LastRefill.Unix())
	})

	t.Run("persistence across restarts", func(t *testing.T) {
		key := "test:persist:1"

		// First store - increment
		store1, err := sqlite.NewSQLiteStore(config)
		require.NoError(t, err)

		val, err := store1.Increment(ctx, key, 5)
		require.NoError(t, err)
		assert.Equal(t, int64(5), val)

		store1.Close(ctx)

		// Second store - should see the same data
		store2, err := sqlite.NewSQLiteStore(config)
		require.NoError(t, err)
		defer store2.Close(ctx)

		val, err = store2.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(5), val)
	})
}