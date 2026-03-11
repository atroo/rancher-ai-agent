package tool

import (
	"encoding/json"
	"log/slog"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/embedding"
	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// RememberPatternTool allows the agent to store an observed pattern in long-term memory.
type RememberPatternTool struct {
	db       *storage.DB
	embedder embedding.Provider // nil = no embeddings
}

func NewRememberPatternTool(db *storage.DB, embedder embedding.Provider) *RememberPatternTool {
	return &RememberPatternTool{db: db, embedder: embedder}
}

func (t *RememberPatternTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "remember_pattern",
		Description: "Store an observed cluster pattern (error, performance issue, scaling event, security concern, config drift) in long-term memory. Use this when you discover a recurring or noteworthy pattern during investigation that may be useful in future conversations.",
		Parameters: schema(map[string]any{
			"category": map[string]any{
				"type":        "string",
				"enum":        []string{"error_pattern", "performance", "scaling", "security", "config_drift"},
				"description": "Category of the observation",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "One-line description of the pattern (used for deduplication and semantic search)",
			},
			"details": map[string]any{
				"type":        "string",
				"description": "Detailed explanation with evidence (metric values, error messages, etc.)",
			},
			"severity": map[string]any{
				"type":        "string",
				"enum":        []string{"info", "warning", "critical"},
				"description": "Severity level",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Kubernetes namespace scope (empty for cluster-wide)",
			},
			"resource": map[string]any{
				"type":        "string",
				"description": "Affected resource (e.g. deployment/payments, node/worker-3)",
			},
		}, []string{"category", "summary", "details", "severity"}),
	}
}

func (t *RememberPatternTool) Execute(tc agent.ToolContext) agent.ToolResult {
	entry := &storage.MemoryEntry{
		Category:  stringParam(tc.Params, "category"),
		Summary:   stringParam(tc.Params, "summary"),
		Details:   stringParam(tc.Params, "details"),
		Severity:  stringParam(tc.Params, "severity"),
		Namespace: stringParam(tc.Params, "namespace"),
		Resource:  stringParam(tc.Params, "resource"),
	}

	if entry.Category == "" || entry.Summary == "" {
		return agent.ToolResult{Content: "category and summary are required", IsError: true}
	}

	// Generate embedding from summary for semantic search
	if t.embedder != nil {
		vec, err := t.embedder.Embed(tc.Ctx, entry.Summary)
		if err != nil {
			slog.Warn("failed to generate embedding for memory entry", "error", err)
			// Continue without embedding — text search still works
		} else {
			entry.Embedding = vec
		}
	}

	id, err := t.db.StoreMemory(entry)
	if err != nil {
		return agent.ToolResult{Content: "failed to store memory: " + err.Error(), IsError: true}
	}

	result := map[string]any{
		"stored":   true,
		"memoryId": id,
		"message":  "Pattern stored in long-term memory. It will be available in future conversations.",
	}
	data, _ := json.Marshal(result)
	return agent.ToolResult{Content: string(data)}
}
