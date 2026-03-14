package config

import (
	"fmt"
	"os"
	"time"
)

// Datasource provider constants.
const (
	// Logs
	LogsProviderKubernetes = "kubernetes" // built-in, no config needed

	// Metrics
	MetricsProviderPrometheus = "prometheus"

	// Traces
	TracesProviderTempo = "tempo"
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

	// Datasources — each category has a provider and provider-specific config.
	Logs    LogsConfig
	Metrics MetricsConfig
	Traces  TracesConfig
}

// LogsConfig configures the logs datasource.
type LogsConfig struct {
	Provider string // "kubernetes" (default, always available)
}

// MetricsConfig configures the metrics datasource.
type MetricsConfig struct {
	Provider string // "prometheus" or "" (disabled)

	// Prometheus-specific
	PrometheusURL string
}

// TracesConfig configures the traces datasource.
type TracesConfig struct {
	Provider string // "tempo" or "" (disabled)

	// Tempo-specific
	TempoURL string
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

		Logs: LogsConfig{
			Provider: envOrDefault("LOGS_PROVIDER", LogsProviderKubernetes),
		},
		Metrics: MetricsConfig{
			Provider:      envOrDefault("METRICS_PROVIDER", ""),
			PrometheusURL: envOrDefault("PROMETHEUS_URL", "http://rancher-monitoring-prometheus.cattle-monitoring-system:9090"),
		},
		Traces: TracesConfig{
			Provider: envOrDefault("TRACES_PROVIDER", ""),
			TempoURL: envOrDefault("TEMPO_URL", "http://tempo-query-frontend.cattle-monitoring-system:3200"),
		},
	}

	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY environment variable is required")
	}

	return cfg, nil
}

// MetricsEnabled returns true if a metrics provider is configured.
func (c *Config) MetricsEnabled() bool {
	return c.Metrics.Provider != ""
}

// TracesEnabled returns true if a traces provider is configured.
func (c *Config) TracesEnabled() bool {
	return c.Traces.Provider != ""
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
