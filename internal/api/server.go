package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/config"
	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/telemetry"
)

// Config represents server configuration
type Config struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Engine       *limiter.Engine
	Config       *config.Config
	Metrics      *telemetry.Metrics
}

// Server represents the HTTP server
type Server struct {
	config   *Config
	server   *http.Server
	router   *Router
}

// NewServer creates a new server
func NewServer(cfg *Config) *Server {
	router := NewRouter(cfg.Engine, cfg.Config, cfg.Metrics)
	
	handler := router.Setup()
	
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{
		config: cfg,
		server: server,
		router: router,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}