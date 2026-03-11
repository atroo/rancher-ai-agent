package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

type VFSQueryJSONTool struct{}

func NewVFSQueryJSONTool() *VFSQueryJSONTool { return &VFSQueryJSONTool{} }

func (t *VFSQueryJSONTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "query_json",
		Description: "Extract values from a stored JSON file using a dot-path expression. Use [*] to iterate arrays. Examples: 'data.result', 'data.result.[*].metric.__name__', 'data.result.[*].value'.",
		Parameters: schema(map[string]any{
			"path":       map[string]any{"type": "string", "description": "File path to query"},
			"jsonPath":   map[string]any{"type": "string", "description": "Dot-separated path into the JSON structure. Use [*] for arrays, [N] for specific index."},
			"maxResults": map[string]any{"type": "integer", "description": "Maximum array items to return when using [*]. Default 50."},
		}, []string{"path", "jsonPath"}),
	}
}

func (t *VFSQueryJSONTool) Execute(tc agent.ToolContext) agent.ToolResult {
	content, err := tc.VFS.QueryJSON(
		stringParam(tc.Params, "path"),
		stringParam(tc.Params, "jsonPath"),
		intParam(tc.Params, "maxResults"),
	)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}
	return agent.ToolResult{Content: content}
}
