package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/WilliamN06/rate-limiter/internal/api/handlers"
	"github.com/WilliamN06/rate-limiter/internal/api/middleware"
	"github.com/WilliamN06/rate-limiter/internal/config"
	"github.com/WilliamN06/rate-limiter/internal/limiter"
	"github.com/WilliamN06/rate-limiter/internal/telemetry"
)

// Router sets up the HTTP routes
type Router struct {
	engine  *limiter.Engine
	config  *config.Config
	metrics *telemetry.Metrics
}

// NewRouter creates a new router
func NewRouter(engine *limiter.Engine, cfg *config.Config, metrics *telemetry.Metrics) *Router {
	return &Router{
		engine:  engine,
		config:  cfg,
		metrics: metrics,
	}
}

// Setup configures all routes and middleware
func (rt *Router) Setup() http.Handler {
	// Create chi router
	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.Logging())
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())

	// Metrics middleware (must come after logging)
	if rt.config.Telemetry.MetricsEnabled {
		router.Use(middleware.Metrics(rt.metrics))
	}

	// Health check endpoints (no rate limiting)
	healthHandler := handlers.NewHealthHandler()
	router.Get("/health", healthHandler.Health)
	router.Get("/ready", healthHandler.Ready)

	// Metrics endpoint
	if rt.config.Telemetry.MetricsEnabled {
		router.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	// API v1 routes
	router.Route("/v1", func(r chi.Router) {
		// Check endpoint with rate limiting
		checkHandler := handlers.NewCheckHandler(rt.engine, rt.config)
		r.Post("/check", checkHandler.Check)

		// Config endpoints
		configHandler := handlers.NewConfigHandler(rt.engine, rt.config)
		r.Get("/config", configHandler.GetConfig)
	})

	return router
}