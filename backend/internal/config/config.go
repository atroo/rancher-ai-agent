package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// Server
	ListenAddr string

	// LLM
	LLMProvider string
	LLMModel    string
	LLMAPIKey   string

	// Embeddings
	EmbeddingBaseURL    string
	EmbeddingAPIKey     string
	EmbeddingModel      string
	EmbeddingDimensions int

	// Data sources
	PrometheusURL string
	TempoURL      string
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:          envOrDefault("LISTEN_ADDR", ":8080"),
		LLMProvider:         envOrDefault("LLM_PROVIDER", "anthropic"),
		LLMModel:            envOrDefault("LLM_MODEL", "claude-sonnet-4-6"),
		LLMAPIKey:           os.Getenv("LLM_API_KEY"),
		EmbeddingBaseURL:    envOrDefault("EMBEDDING_BASE_URL", "https://api.voyageai.com/v1"),
		EmbeddingAPIKey:     os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingModel:      envOrDefault("EMBEDDING_MODEL", "voyage-3-lite"),
		EmbeddingDimensions: envOrDefaultInt("EMBEDDING_DIMENSIONS", 512),
		PrometheusURL:       envOrDefault("PROMETHEUS_URL", "http://rancher-monitoring-prometheus.cattle-monitoring-system:9090"),
		TempoURL:            envOrDefault("TEMPO_URL", "http://tempo-query-frontend.cattle-monitoring-system:3200"),
	}

	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY environment variable is required")
	}

	return cfg, nil
}

// RetryBaseDelay returns the base delay for LLM API retries.
func (c *Config) RetryBaseDelay() time.Duration {
	return time.Second
}

// SessionTTL returns the session TTL.
func (c *Config) SessionTTL() time.Duration {
	return 30 * time.Minute
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}
