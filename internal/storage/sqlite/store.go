package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/WilliamN06/rate-limiter/internal/storage"
)

// SQLiteStore implements storage.Store using SQLite
type SQLiteStore struct {
	db          *sql.DB
	mu          sync.RWMutex
	flushTicker *time.Ticker
	stopFlush   chan struct{}
	config      *Config
}

// Config configures the SQLite store
type Config struct {
	DBPath         string
	FlushInterval  time.Duration
	FlushThreshold int
	MaxOpenConns   int
	MaxIdleConns   int
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DBPath:         "./data/rate-limiter.db",
		FlushInterval:  5 * time.Second,
		FlushThreshold: 1000,
		MaxOpenConns:   10,
		MaxIdleConns:   5,
	}
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(config *Config) (*SQLiteStore, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Open database with connection pool settings
	db, err := sql.Open("sqlite3", fmt.Sprintf(
		"%s?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_sync=NORMAL",
		config.DBPath,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &SQLiteStore{
		db:        db,
		config:    config,
		stopFlush: make(chan struct{}),
	}

	// Run migrations
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Start flush ticker
	if config.FlushInterval > 0 {
		store.flushTicker = time.NewTicker(config.FlushInterval)
		go store.flushLoop()
	}

	return store, nil
}

// migrate creates the necessary tables
func (s *SQLiteStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS counters (
			key TEXT PRIMARY KEY,
			value INTEGER DEFAULT 0,
			updated_at INTEGER
		)`,

		`CREATE TABLE IF NOT EXISTS timestamps (
			key TEXT,
			timestamp INTEGER,
			PRIMARY KEY (key, timestamp)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_timestamps_key ON timestamps(key)`,

		`CREATE TABLE IF NOT EXISTS token_states (
			key TEXT PRIMARY KEY,
			tokens INTEGER,
			last_refill INTEGER
		)`,

		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,

		`PRAGMA auto_vacuum = INCREMENTAL`,
	}

	for _, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return err
		}
	}

	return nil
}

// flushLoop periodically flushes data to disk
func (s *SQLiteStore) flushLoop() {
	for {
		select {
		case <-s.flushTicker.C:
			if err := s.Flush(context.Background()); err != nil {
				// Log error but continue
				fmt.Printf("Failed to flush: %v\n", err)
			}
		case <-s.stopFlush:
			return
		}
	}
}

// Increment implements CounterStore.Increment
func (s *SQLiteStore) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	// We'll implement this with SQLite directly
	var current int64
	err := s.db.QueryRowContext(ctx, 
		"SELECT value FROM counters WHERE key = ?", key).
		Scan(&current)
	
	if err == sql.ErrNoRows {
		// Insert new counter
		_, err = s.db.ExecContext(ctx,
			"INSERT INTO counters (key, value, updated_at) VALUES (?, ?, ?)",
			key, delta, time.Now().Unix())
		if err != nil {
			return 0, err
		}
		return delta, nil
	}
	if err != nil {
		return 0, err
	}

	// Update existing counter
	newValue := current + delta
	_, err = s.db.ExecContext(ctx,
		"UPDATE counters SET value = ?, updated_at = ? WHERE key = ?",
		newValue, time.Now().Unix(), key)
	if err != nil {
		return 0, err
	}
	return newValue, nil
}

// Get implements CounterStore.Get
func (s *SQLiteStore) Get(ctx context.Context, key string) (int64, error) {
	var value int64
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM counters WHERE key = ?", key).
		Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return value, err
}

// GetCounters implements CounterStore.GetCounters
func (s *SQLiteStore) GetCounters(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM counters")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var key string
		var value int64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// AddTimestamp implements TimestampStore.AddTimestamp
func (s *SQLiteStore) AddTimestamp(ctx context.Context, key string, ts time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO timestamps (key, timestamp) VALUES (?, ?)",
		key, ts.Unix())
	return err
}

// GetTimestamps implements TimestampStore.GetTimestamps
func (s *SQLiteStore) GetTimestamps(ctx context.Context, key string, after time.Time) ([]time.Time, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT timestamp FROM timestamps WHERE key = ? AND timestamp > ? ORDER BY timestamp",
		key, after.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timestamps []time.Time
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		timestamps = append(timestamps, time.Unix(ts, 0))
	}
	return timestamps, nil
}

// PruneTimestamps implements TimestampStore.PruneTimestamps
func (s *SQLiteStore) PruneTimestamps(ctx context.Context, key string, before time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM timestamps WHERE key = ? AND timestamp < ?",
		key, before.Unix())
	return err
}

// GetTokenState implements TokenStore.GetTokenState
func (s *SQLiteStore) GetTokenState(ctx context.Context, key string) (*storage.TokenState, error) {
	var tokens int
	var lastRefill int64
	err := s.db.QueryRowContext(ctx,
		"SELECT tokens, last_refill FROM token_states WHERE key = ?", key).
		Scan(&tokens, &lastRefill)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &storage.TokenState{
		Tokens:     tokens,
		LastRefill: time.Unix(lastRefill, 0),
	}, nil
}

// UpdateTokenState implements TokenStore.UpdateTokenState
func (s *SQLiteStore) UpdateTokenState(ctx context.Context, key string, state *storage.TokenState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO token_states (key, tokens, last_refill) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET tokens = ?, last_refill = ?`,
		key, state.Tokens, state.LastRefill.Unix(),
		state.Tokens, state.LastRefill.Unix())
	return err
}

// Close implements Store.Close
func (s *SQLiteStore) Close(ctx context.Context) error {
	if s.flushTicker != nil {
		s.flushTicker.Stop()
	}
	close(s.stopFlush)

	// Flush before closing
	if err := s.Flush(ctx); err != nil {
		return err
	}
	return s.db.Close()
}

// Flush implements Store.Flush (no-op for SQLite since we write directly)
func (s *SQLiteStore) Flush(ctx context.Context) error {
	// SQLite writes are immediate, but we can force a checkpoint
	_, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Load implements Store.Load (no-op for SQLite since we read directly)
func (s *SQLiteStore) Load(ctx context.Context) error {
	// Data is read directly from SQLite
	return nil
}

// Stats implements Store.Stats
func (s *SQLiteStore) Stats(ctx context.Context) (storage.StoreStats, error) {
	var stats storage.StoreStats

	// Count counters
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM counters").Scan(&stats.Counters)
	if err != nil {
		return stats, err
	}

	// Count timestamp keys
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT key) FROM timestamps").Scan(&stats.TimestampKeys)
	if err != nil {
		return stats, err
	}

	// Count timestamp entries
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM timestamps").Scan(&stats.TimestampEntries)
	if err != nil {
		return stats, err
	}

	// Count token states
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM token_states").Scan(&stats.TokenKeys)
	if err != nil {
		return stats, err
	}

	return stats, nil
}