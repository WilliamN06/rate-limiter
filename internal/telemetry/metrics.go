package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics contains all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	
	// Rate limiter metrics
	RateLimitChecksTotal *prometheus.CounterVec
	RateLimitHitsTotal   *prometheus.CounterVec
	ActiveClients        prometheus.Gauge
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		RateLimitChecksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limiter_checks_total",
				Help: "Total number of rate limit checks",
			},
			[]string{"client_id", "endpoint", "algorithm", "allowed"},
		),
		RateLimitHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limiter_hits_total",
				Help: "Total number of rate limit hits (429 responses)",
			},
			[]string{"client_id", "endpoint"},
		),
		ActiveClients: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "rate_limiter_active_clients",
				Help: "Number of active clients",
			},
		),
	}
	
	return m
}