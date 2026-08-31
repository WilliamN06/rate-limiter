package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/WilliamN06/rate-limiter/internal/config"
	"github.com/WilliamN06/rate-limiter/internal/limiter"
)

// CheckHandler handles the /v1/check endpoint
type CheckHandler struct {
	engine *limiter.Engine
	config *config.Config
}

// CheckRequest represents the request body
type CheckRequest struct {
	ClientID  string `json:"clientId"`
	Endpoint  string `json:"endpoint"`
	Quantity  int    `json:"quantity,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
}

// CheckResponse represents the response body
type CheckResponse struct {
	Allowed    bool   `json:"allowed"`
	Limit      int    `json:"limit"`
	Remaining  int    `json:"remaining"`
	ResetAt    int64  `json:"resetAt"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// NewCheckHandler creates a new check handler
func NewCheckHandler(engine *limiter.Engine, cfg *config.Config) *CheckHandler {
	return &CheckHandler{
		engine: engine,
		config: cfg,
	}
}

// Check handles the rate limit check
func (h *CheckHandler) Check(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON body", err.Error())
		return
	}

	// Validate request
	if req.ClientID == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_CLIENT_ID", "clientId is required", "")
		return
	}

	if req.Endpoint == "" {
		h.writeError(w, http.StatusBadRequest, "MISSING_ENDPOINT", "endpoint is required", "")
		return
	}

	// Set defaults
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// Determine limit and algorithm
	limit := h.config.RateLimiter.DefaultLimit
	window := h.config.RateLimiter.DefaultWindow
	algo := limiter.Algorithm(h.config.RateLimiter.DefaultAlgorithm)

	// Check for client overrides
	for _, override := range h.config.RateLimiter.Overrides {
		if override.ClientID == req.ClientID {
			if override.Limit > 0 {
				limit = override.Limit
			}
			if override.Algorithm != "" {
				algo = limiter.Algorithm(override.Algorithm)
			}
			if override.Window > 0 {
				window = override.Window
			}
			break
		}
	}

	// Allow algorithm override from request
	if req.Algorithm != "" {
		algo = limiter.Algorithm(req.Algorithm)
	}

	// Execute rate limit check
	key := req.ClientID + ":" + req.Endpoint
	result, err := h.engine.CheckWithAlgorithm(r.Context(), key, limit, window, algo)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check rate limit", err.Error())
		return
	}

	// Set rate limit headers
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

	// Prepare response
	response := CheckResponse{
		Allowed:   result.Allowed,
		Limit:     result.Limit,
		Remaining: result.Remaining,
		ResetAt:   result.ResetAt.Unix(),
	}

	if !result.Allowed {
		w.Header().Set("Retry-After", strconv.Itoa(result.RetryAfter))
		response.RetryAfter = result.RetryAfter
		w.WriteHeader(http.StatusTooManyRequests)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// writeError writes an error response
func (h *CheckHandler) writeError(w http.ResponseWriter, status int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	})
}