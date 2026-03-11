package datasource

// Sources groups all data source clients for the agent.
type Sources struct {
	Prometheus *PrometheusClient
	Tempo      *TempoClient
	Kubernetes *KubernetesClient
}

// NewSources initializes all data source clients.
func NewSources(prometheusURL, tempoURL string) (*Sources, error) {
	k8s, err := NewKubernetesClient()
	if err != nil {
		return nil, err
	}

	return &Sources{
		Prometheus: NewPrometheusClient(prometheusURL),
		Tempo:      NewTempoClient(tempoURL),
		Kubernetes: k8s,
	}, nil
}
