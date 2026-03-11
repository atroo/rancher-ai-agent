package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

// SpawnSubagentTool creates a child agent run with a fresh context for a focused task.
// The sub-agent shares the same tools and LLM provider but gets a fresh VFS and message history.
// Results are collected and returned to the parent agent as a single tool result.
type SpawnSubagentTool struct {
	agentFactory func() *agent.Agent
}

// NewSpawnSubagentTool creates the tool. The factory function should return a configured
// agent (same provider, registry, middleware) suitable for sub-agent use.
func NewSpawnSubagentTool(factory func() *agent.Agent) *SpawnSubagentTool {
	return &SpawnSubagentTool{agentFactory: factory}
}

func (t *SpawnSubagentTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name: "spawn_subagent",
		Description: `Spawn a focused sub-agent to investigate a specific sub-task independently.
The sub-agent gets a fresh conversation context and VFS, so it won't pollute the main conversation.
Use this for:
- Deep-dive investigations (e.g., "analyze all pods in namespace X")
- Parallel data collection (e.g., "gather metrics for the last 24h")
- Complex multi-step queries that would clutter the main conversation

The sub-agent's final text response is returned as this tool's result.`,
		Parameters: schema(map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "Clear, specific task description for the sub-agent. Include all necessary context (namespaces, resource names, time ranges, etc.).",
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Optional Rancher context to scope the sub-agent",
				"properties": map[string]any{
					"clusterId":    map[string]any{"type": "string"},
					"namespace":    map[string]any{"type": "string"},
					"resourceType": map[string]any{"type": "string"},
					"resourceName": map[string]any{"type": "string"},
				},
			},
		}, []string{"task"}),
	}
}

func (t *SpawnSubagentTool) Execute(tc agent.ToolContext) agent.ToolResult {
	task := stringParam(tc.Params, "task")
	if task == "" {
		return agent.ToolResult{Content: "task description is required", IsError: true}
	}

	var convCtx agent.ConversationContext
	if ctxMap, ok := tc.Params["context"].(map[string]any); ok {
		convCtx.ClusterID = stringParam(ctxMap, "clusterId")
		convCtx.Namespace = stringParam(ctxMap, "namespace")
		convCtx.ResourceType = stringParam(ctxMap, "resourceType")
		convCtx.ResourceName = stringParam(ctxMap, "resourceName")
	}

	subAgent := t.agentFactory()

	// Create a timeout context for the sub-agent (max 2 minutes)
	ctx, cancel := context.WithTimeout(tc.Ctx, 2*time.Minute)
	defer cancel()

	sessionID := fmt.Sprintf("subagent_%d", time.Now().UnixNano())

	events := subAgent.Run(ctx, agent.RunOptions{
		SessionID:   sessionID,
		UserMessage: task,
		ConvCtx:     convCtx,
	})

	// Collect the sub-agent's text output
	var textParts []string
	var toolsSummary []string
	var lastError string

	for event := range events {
		switch event.Type {
		case agent.EventTextDelta:
			d := event.Data.(agent.TextDeltaData)
			textParts = append(textParts, d.Delta)

		case agent.EventToolCallStart:
			d := event.Data.(agent.ToolCallStartData)
			toolsSummary = append(toolsSummary, d.ToolName)

		case agent.EventError:
			d := event.Data.(agent.ErrorData)
			lastError = d.Error
		}
	}

	fullText := strings.Join(textParts, "")

	if fullText == "" && lastError != "" {
		return agent.ToolResult{
			Content: fmt.Sprintf("Sub-agent failed: %s", lastError),
			IsError: true,
		}
	}

	result := map[string]any{
		"response":  fullText,
		"toolsUsed": toolsSummary,
	}
	data, _ := json.Marshal(result)
	return agent.ToolResult{Content: string(data)}
}
