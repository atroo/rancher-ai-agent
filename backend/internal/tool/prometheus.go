package tool

import (
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type PrometheusTool struct {
	client *datasource.PrometheusClient
}

func NewPrometheusTool(client *datasource.PrometheusClient) *PrometheusTool {
	return &PrometheusTool{client: client}
}

func (t *PrometheusTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "query_prometheus",
		Description: "Execute a PromQL query against the cluster's Prometheus. Results are stored in the virtual filesystem; you receive a summary with the file path. Use read_file or query_json to inspect the full data.",
		Parameters: schema(map[string]any{
			"query": map[string]any{"type": "string", "description": "The PromQL query to execute"},
			"time":  map[string]any{"type": "string", "description": "Evaluation timestamp (RFC3339). Omit for current time."},
			"start": map[string]any{"type": "string", "description": "Range query start (RFC3339). If set, 'end' and 'step' are also required."},
			"end":   map[string]any{"type": "string", "description": "Range query end (RFC3339)."},
			"step":  map[string]any{"type": "string", "description": "Range query step (e.g., '15s', '1m', '5m')."},
		}, []string{"query"}),
	}
}

func (t *PrometheusTool) Execute(tc agent.ToolContext) agent.ToolResult {
	raw, err := t.client.Query(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}

	f := tc.VFS.Write("prometheus", fmt.Sprintf("PromQL: %s", stringParam(tc.Params, "query")), raw)
	return agent.ToolResult{Content: SummarizePrometheusResult(raw, f.Path)}
}
