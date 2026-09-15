package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/api"
	"github.com/WilliamN06/rate-limiter/internal/config"
	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/limiter/algorithms"
	"github.com/WilliamN06/rate-limiter/internal/storage/memory"
	"github.com/WilliamN06/rate-limiter/internal/storage/sqlite"
	"github.com/WilliamN06/rate-limiter/internal/telemetry"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Parse config path from environment or use default
	configPath := os.Getenv("RATE_LIMITER_CONFIG")
	if configPath == "" {
		configPath = "./configs/config.yaml"
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup logger
	logger := telemetry.NewLogger(&cfg.Telemetry)
	slog.SetDefault(logger)

	slog.Info("starting rate limiter service",
		"version", version,
		"commit", commit,
		"config", configPath,
	)

	// Initialize storage based on configuration
	var store interface {
		// All store interfaces
	}

	if cfg.Persistence.Enabled {
		slog.Info("using SQLite storage with persistence", "db_path", cfg.Persistence.DBPath)
		
		// Create data directory if it doesn't exist
		if err := os.MkdirAll("./data", 0755); err != nil {
			slog.Error("failed to create data directory", "error", err)
			os.Exit(1)
		}

		sqliteConfig := &sqlite.Config{
			DBPath:         cfg.Persistence.DBPath,
			FlushInterval:  cfg.Persistence.FlushInterval,
			FlushThreshold: cfg.Persistence.FlushThreshold,
			MaxOpenConns:   10,
			MaxIdleConns:   5,
		}
		
		store, err = sqlite.NewSQLiteStore(sqliteConfig)
		if err != nil {
			slog.Error("failed to create SQLite store", "error", err)
			os.Exit(1)
		}
		
		slog.Info("SQLite storage initialized successfully")
	} else {
		slog.Info("using in-memory storage (no persistence)")
		
		memoryConfig := &memory.StoreConfig{
			MaxKeys:          cfg.Storage.Memory.MaxClients,
			RingBufferCapacity: 1000,
			CleanupInterval: 5 * time.Minute,
		}
		store = memory.NewInMemoryStoreWithConfig(memoryConfig)
	}

	// Initialize rate limiter engine
	algorithmsMap := make(map[limiter.Algorithm]limiter.Limiter)
	algorithmsMap[limiter.FixedWindow] = algorithms.NewFixedWindowLimiter(store)
	algorithmsMap[limiter.SlidingWindow] = algorithms.NewSlidingWindowLimiter(store)
	algorithmsMap[limiter.TokenBucket] = algorithms.NewTokenBucketLimiter(store)

	engine, err := limiter.NewEngine(&limiter.EngineConfig{
		Store:            store,
		DefaultAlgorithm: limiter.FixedWindow,
		Algorithms:       algorithmsMap,
	})
	if err != nil {
		slog.Error("failed to create rate limiter engine", "error", err)
		os.Exit(1)
	}

	// Initialize metrics
	metrics := telemetry.NewMetrics()

	// Setup API server
	server := api.NewServer(&api.Config{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		Engine:       engine,
		Config:       cfg,
		Metrics:      metrics,
	})

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil {
			slog.Error("server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("shutdown complete")
}