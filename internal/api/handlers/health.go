package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	startTime time.Time
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
	}
}

// Health handles the /health endpoint
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
		Uptime:    time.Since(h.startTime).String(),
		Version:   "dev",
	})
}

// Ready handles the /ready endpoint
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Check if the service is ready to accept traffic
	// For now, always return ready
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "ready",
		Timestamp: time.Now().Unix(),
		Uptime:    time.Since(h.startTime).String(),
		Version:   "dev",
	})
}