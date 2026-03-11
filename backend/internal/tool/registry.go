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
	DB           *storage.DB            // nil disables memory tools
	Embedder     embedding.Provider     // nil disables semantic search (text fallback)
	AgentFactory func() *agent.Agent    // nil disables sub-agent tool
}

// DefaultRegistry creates a ToolRegistry with all built-in tools registered.
func DefaultRegistry(deps RegistryDeps) *agent.ToolRegistry {
	r := agent.NewToolRegistry()

	// Data source tools
	r.Register(NewPrometheusTool(deps.Sources.Prometheus))
	r.Register(NewTempoSearchTool(deps.Sources.Tempo))
	r.Register(NewTempoGetTraceTool(deps.Sources.Tempo))
	r.Register(NewPodLogsTool(deps.Sources.Kubernetes))
	r.Register(NewK8sEventsTool(deps.Sources.Kubernetes))
	r.Register(NewResourceStatusTool(deps.Sources.Kubernetes))
	r.Register(NewListWorkloadsTool(deps.Sources.Kubernetes))

	// VFS tools
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
