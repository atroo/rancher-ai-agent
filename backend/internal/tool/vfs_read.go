package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

type VFSReadFileTool struct{}

func NewVFSReadFileTool() *VFSReadFileTool { return &VFSReadFileTool{} }

func (t *VFSReadFileTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "read_file",
		Description: "Read the content of a file in the virtual filesystem. Supports line-based offset and limit for large files.",
		Parameters: schema(map[string]any{
			"path":   map[string]any{"type": "string", "description": "File path (from list_files or a previous tool result)"},
			"offset": map[string]any{"type": "integer", "description": "Start reading from this line number (0-based). Default 0."},
			"limit":  map[string]any{"type": "integer", "description": "Maximum number of lines to read. Default: all lines. Use this for large files."},
		}, []string{"path"}),
	}
}

func (t *VFSReadFileTool) Execute(tc agent.ToolContext) agent.ToolResult {
	content, err := tc.VFS.Read(
		stringParam(tc.Params, "path"),
		intParam(tc.Params, "offset"),
		intParam(tc.Params, "limit"),
	)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}
	return agent.ToolResult{Content: content}
}
