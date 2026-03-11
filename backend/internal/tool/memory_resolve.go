package tool

import (
	"encoding/json"
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// ResolvePatternTool allows the agent to mark a memory entry as resolved.
type ResolvePatternTool struct {
	db *storage.DB
}

func NewResolvePatternTool(db *storage.DB) *ResolvePatternTool {
	return &ResolvePatternTool{db: db}
}

func (t *ResolvePatternTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "resolve_pattern",
		Description: "Mark a previously stored pattern as resolved. Use this when you confirm an issue has been fixed.",
		Parameters: schema(map[string]any{
			"memory_id": map[string]any{
				"type":        "integer",
				"description": "ID of the memory entry to resolve (from recall_patterns results)",
			},
		}, []string{"memory_id"}),
	}
}

func (t *ResolvePatternTool) Execute(tc agent.ToolContext) agent.ToolResult {
	id := intParam(tc.Params, "memory_id")
	if id == 0 {
		return agent.ToolResult{Content: "memory_id is required", IsError: true}
	}

	if err := t.db.ResolveMemory(id); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("failed to resolve memory %d: %s", id, err), IsError: true}
	}

	result := map[string]any{"resolved": true, "memoryId": id}
	data, _ := json.Marshal(result)
	return agent.ToolResult{Content: string(data)}
}
