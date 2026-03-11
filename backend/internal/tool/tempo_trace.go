package tool

import (
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type TempoGetTraceTool struct {
	client *datasource.TempoClient
}

func NewTempoGetTraceTool(client *datasource.TempoClient) *TempoGetTraceTool {
	return &TempoGetTraceTool{client: client}
}

func (t *TempoGetTraceTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "get_trace",
		Description: "Fetch a complete trace by its trace ID from Grafana Tempo. The full trace is stored in the virtual filesystem; you receive a summary of spans. Use query_json to drill into specific spans.",
		Parameters: schema(map[string]any{
			"traceId": map[string]any{"type": "string", "description": "The trace ID (hex string)"},
		}, []string{"traceId"}),
	}
}

func (t *TempoGetTraceTool) Execute(tc agent.ToolContext) agent.ToolResult {
	traceID := stringParam(tc.Params, "traceId")
	raw, err := t.client.GetTrace(tc.Ctx, traceID)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}

	f := tc.VFS.Write("tempo_trace", fmt.Sprintf("Trace: %s", traceID), raw)
	return agent.ToolResult{Content: SummarizeTraceResult(raw, f.Path)}
}
