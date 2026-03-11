package tool

import (
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type TempoSearchTool struct {
	client *datasource.TempoClient
}

func NewTempoSearchTool(client *datasource.TempoClient) *TempoSearchTool {
	return &TempoSearchTool{client: client}
}

func (t *TempoSearchTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "search_traces",
		Description: "Search Grafana Tempo for traces matching criteria. Results are stored in the virtual filesystem; you receive a summary with the file path. Use read_file or query_json to inspect individual traces.",
		Parameters: schema(map[string]any{
			"query": map[string]any{"type": "string", "description": "TraceQL query (e.g., '{resource.service.name=\"my-service\" && duration>2s}')"},
			"limit": map[string]any{"type": "integer", "description": "Maximum number of traces to return. Default 20."},
			"start": map[string]any{"type": "string", "description": "Search window start as Unix epoch seconds."},
			"end":   map[string]any{"type": "string", "description": "Search window end as Unix epoch seconds."},
		}, []string{"query"}),
	}
}

func (t *TempoSearchTool) Execute(tc agent.ToolContext) agent.ToolResult {
	raw, err := t.client.Search(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}

	f := tc.VFS.Write("tempo_search", fmt.Sprintf("TraceQL: %s", stringParam(tc.Params, "query")), raw)
	return agent.ToolResult{Content: SummarizeTempoSearchResult(raw, f.Path)}
}
