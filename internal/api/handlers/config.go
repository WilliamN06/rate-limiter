package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/WilliamN06/rate-limiter/internal/config"
	"github.com/WilliamN06/rate-limiter/internal/limiter"
)

// ConfigHandler handles configuration endpoints
type ConfigHandler struct {
	engine *limiter.Engine
	config *config.Config
}

// ConfigResponse represents the configuration response
type ConfigResponse struct {
	DefaultLimit      int      `json:"defaultLimit"`
	DefaultWindow     string   `json:"defaultWindow"`
	DefaultAlgorithm  string   `json:"defaultAlgorithm"`
	SupportedAlgorithms []string `json:"supportedAlgorithms"`
	Overrides         []OverrideResponse `json:"overrides,omitempty"`
}

// OverrideResponse represents a client override in the response
type OverrideResponse struct {
	ClientID  string `json:"clientId"`
	Limit     int    `json:"limit"`
	Algorithm string `json:"algorithm"`
	Window    string `json:"window"`
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(engine *limiter.Engine, cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{
		engine: engine,
		config: cfg,
	}
}

// GetConfig handles the /v1/config endpoint
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Get supported algorithms
	algorithms := h.engine.GetSupportedAlgorithms()
	algoStrings := make([]string, len(algorithms))
	for i, algo := range algorithms {
		algoStrings[i] = string(algo)
	}

	// Build overrides
	overrides := make([]OverrideResponse, len(h.config.RateLimiter.Overrides))
	for i, override := range h.config.RateLimiter.Overrides {
		overrides[i] = OverrideResponse{
			ClientID:  override.ClientID,
			Limit:     override.Limit,
			Algorithm: override.Algorithm,
			Window:    override.Window.String(),
		}
	}

	response := ConfigResponse{
		DefaultLimit:      h.config.RateLimiter.DefaultLimit,
		DefaultWindow:     h.config.RateLimiter.DefaultWindow.String(),
		DefaultAlgorithm:  h.config.RateLimiter.DefaultAlgorithm,
		SupportedAlgorithms: algoStrings,
		Overrides:         overrides,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}