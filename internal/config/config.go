package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	RateLimiter RateLimiterConfig `mapstructure:"rate_limiter"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Telemetry   TelemetryConfig   `mapstructure:"telemetry"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// RateLimiterConfig contains rate limiter settings
type RateLimiterConfig struct {
	DefaultLimit      int           `mapstructure:"default_limit"`
	DefaultWindow     time.Duration `mapstructure:"default_window"`
	DefaultAlgorithm  string        `mapstructure:"default_algorithm"`
	Overrides         []Override    `mapstructure:"overrides"`
}

// Override represents a per-client override
type Override struct {
	ClientID  string        `mapstructure:"client_id"`
	Limit     int           `mapstructure:"limit"`
	Algorithm string        `mapstructure:"algorithm"`
	Window    time.Duration `mapstructure:"window"`
}

// StorageConfig contains storage settings
type StorageConfig struct {
	Memory struct {
		MaxClients int           `mapstructure:"max_clients"`
		DefaultTTL time.Duration `mapstructure:"default_ttl"`
	} `mapstructure:"memory"`
	Persistence struct {
		Enabled          bool          `mapstructure:"enabled"`
		DBPath           string        `mapstructure:"db_path"`
		FlushInterval    time.Duration `mapstructure:"flush_interval"`
		FlushThreshold   int           `mapstructure:"flush_threshold"`
		CheckpointInterval time.Duration `mapstructure:"checkpoint_interval"`
	} `mapstructure:"persistence"`
}

// TelemetryConfig contains telemetry settings
type TelemetryConfig struct {
	MetricsEnabled bool   `mapstructure:"metrics_enabled"`
	LogLevel       string `mapstructure:"log_level"`
	LogFormat      string `mapstructure:"log_format"`
}

// Load loads the configuration from file and environment
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "5s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "120s")
	v.SetDefault("rate_limiter.default_limit", 100)
	v.SetDefault("rate_limiter.default_window", "60s")
	v.SetDefault("rate_limiter.default_algorithm", "fixed_window")
	v.SetDefault("storage.memory.max_clients", 10000)
	v.SetDefault("storage.memory.default_ttl", "10m")
	v.SetDefault("telemetry.metrics_enabled", true)
	v.SetDefault("telemetry.log_level", "info")
	v.SetDefault("telemetry.log_format", "json")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}