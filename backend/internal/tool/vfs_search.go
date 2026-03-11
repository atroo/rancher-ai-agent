package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

type VFSSearchTool struct{}

func NewVFSSearchTool() *VFSSearchTool { return &VFSSearchTool{} }

func (t *VFSSearchTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "search_in_file",
		Description: "Search for a regex pattern or substring in a stored file. Returns matching lines with line numbers.",
		Parameters: schema(map[string]any{
			"path":       map[string]any{"type": "string", "description": "File path to search in"},
			"pattern":    map[string]any{"type": "string", "description": "Regex pattern or substring to search for (e.g., 'error', 'OOM', 'status.*500')"},
			"maxResults": map[string]any{"type": "integer", "description": "Maximum number of matching lines to return. Default 50."},
		}, []string{"path", "pattern"}),
	}
}

func (t *VFSSearchTool) Execute(tc agent.ToolContext) agent.ToolResult {
	content, err := tc.VFS.Search(
		stringParam(tc.Params, "path"),
		stringParam(tc.Params, "pattern"),
		intParam(tc.Params, "maxResults"),
	)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}
	return agent.ToolResult{Content: content}
}
