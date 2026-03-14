package datasource

import "github.com/atroo/rancher-ai-assistant/backend/internal/config"

// Sources groups all data source clients for the agent.
// Nil fields mean the datasource is not configured.
type Sources struct {
	Prometheus *PrometheusClient
	Tempo      *TempoClient
	Kubernetes *KubernetesClient
}

// NewSources initializes data source clients based on config.
// Only creates clients for configured providers.
func NewSources(cfg *config.Config) (*Sources, error) {
	s := &Sources{}

	// Logs: Kubernetes is always available (runs in-cluster)
	if cfg.Logs.Provider == config.LogsProviderKubernetes {
		k8s, err := NewKubernetesClient()
		if err != nil {
			return nil, err
		}
		s.Kubernetes = k8s
	}

	// Metrics
	if cfg.Metrics.Provider == config.MetricsProviderPrometheus {
		s.Prometheus = NewPrometheusClient(cfg.Metrics.PrometheusURL)
	}

	// Traces
	if cfg.Traces.Provider == config.TracesProviderTempo {
		s.Tempo = NewTempoClient(cfg.Traces.TempoURL)
	}

	return s, nil
}
