package tool

import (
	"encoding/json"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

type VFSListTool struct{}

func NewVFSListTool() *VFSListTool { return &VFSListTool{} }

func (t *VFSListTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "list_files",
		Description: "List all files in the virtual filesystem from previous tool results. Shows path, source tool, description, and size.",
		Parameters:  schema(map[string]any{}, nil),
	}
}

func (t *VFSListTool) Execute(tc agent.ToolContext) agent.ToolResult {
	files := tc.VFS.List()
	if len(files) == 0 {
		return agent.ToolResult{Content: "No files stored yet. Run a data source tool first."}
	}
	out, _ := json.MarshalIndent(files, "", "  ")
	return agent.ToolResult{Content: string(out)}
}
