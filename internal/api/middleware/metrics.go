package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/WilliamN06/rate-limiter/internal/telemetry"
)

// Metrics collects HTTP metrics
func Metrics(metrics *telemetry.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := NewResponseWriter(w)
			
			next.ServeHTTP(ww, r)
			
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())
			
			metrics.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
		})
	}
}