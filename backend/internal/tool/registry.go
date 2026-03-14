package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
	"github.com/atroo/rancher-ai-assistant/backend/internal/embedding"
	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// RegistryDeps holds dependencies needed by tools that go beyond datasources.
type RegistryDeps struct {
	Sources      *datasource.Sources
	DB           *storage.DB         // nil disables memory tools
	Embedder     embedding.Provider  // nil disables semantic search (text fallback)
	AgentFactory func() *agent.Agent // nil disables sub-agent tool
}

// DefaultRegistry creates a ToolRegistry with tools for configured providers only.
func DefaultRegistry(deps RegistryDeps) *agent.ToolRegistry {
	r := agent.NewToolRegistry()

	// Logs tools (requires Kubernetes client)
	if deps.Sources.Kubernetes != nil {
		r.Register(NewPodLogsTool(deps.Sources.Kubernetes))
		r.Register(NewK8sEventsTool(deps.Sources.Kubernetes))
		r.Register(NewResourceStatusTool(deps.Sources.Kubernetes))
		r.Register(NewListWorkloadsTool(deps.Sources.Kubernetes))
	}

	// Metrics tools (requires Prometheus client)
	if deps.Sources.Prometheus != nil {
		r.Register(NewPrometheusTool(deps.Sources.Prometheus))
	}

	// Traces tools (requires Tempo client)
	if deps.Sources.Tempo != nil {
		r.Register(NewTempoSearchTool(deps.Sources.Tempo))
		r.Register(NewTempoGetTraceTool(deps.Sources.Tempo))
	}

	// VFS tools (always available)
	r.Register(NewVFSListTool())
	r.Register(NewVFSReadFileTool())
	r.Register(NewVFSSearchTool())
	r.Register(NewVFSQueryJSONTool())

	// Memory tools (require DB)
	if deps.DB != nil {
		r.Register(NewRememberPatternTool(deps.DB, deps.Embedder))
		r.Register(NewRecallPatternsTool(deps.DB, deps.Embedder))
		r.Register(NewResolvePatternTool(deps.DB))
	}

	// Sub-agent tool (requires agent factory)
	if deps.AgentFactory != nil {
		r.Register(NewSpawnSubagentTool(deps.AgentFactory))
	}

	return r
}
