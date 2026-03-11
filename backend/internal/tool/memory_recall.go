package tool

import (
	"encoding/json"
	"log/slog"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/embedding"
	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// RecallPatternsTool allows the agent to search long-term memory for previously stored patterns.
// Supports both semantic search (via embeddings) and text-based fallback.
type RecallPatternsTool struct {
	db       *storage.DB
	embedder embedding.Provider // nil = text search only
}

func NewRecallPatternsTool(db *storage.DB, embedder embedding.Provider) *RecallPatternsTool {
	return &RecallPatternsTool{db: db, embedder: embedder}
}

func (t *RecallPatternsTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "recall_patterns",
		Description: "Search long-term memory for previously observed cluster patterns using semantic similarity. Returns matching entries ranked by relevance. Use this to check if a current issue has been seen before.",
		Parameters: schema(map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language search query (e.g., 'OOM kills in payment service', 'high CPU on worker nodes')",
			},
			"category": map[string]any{
				"type":        "string",
				"enum":        []string{"error_pattern", "performance", "scaling", "security", "config_drift"},
				"description": "Filter by category (optional)",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Filter by namespace (optional)",
			},
			"include_resolved": map[string]any{
				"type":        "boolean",
				"description": "Include resolved patterns (default: false)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results to return (default: 20)",
			},
		}, nil),
	}
}

func (t *RecallPatternsTool) Execute(tc agent.ToolContext) agent.ToolResult {
	query := stringParam(tc.Params, "query")
	category := stringParam(tc.Params, "category")
	namespace := stringParam(tc.Params, "namespace")
	includeResolved := boolParam(tc.Params, "include_resolved")
	limit := intParam(tc.Params, "limit")

	// Try semantic search first if we have an embedder and a query
	if t.embedder != nil && query != "" {
		queryVec, err := t.embedder.Embed(tc.Ctx, query)
		if err != nil {
			slog.Warn("failed to embed query, falling back to text search", "error", err)
		} else {
			entries, err := t.db.SemanticSearchMemory(queryVec, category, namespace, includeResolved, limit, 0.3)
			if err != nil {
				return agent.ToolResult{Content: "semantic search failed: " + err.Error(), IsError: true}
			}

			result := map[string]any{
				"count":      len(entries),
				"searchType": "semantic",
				"entries":    entries,
			}
			data, _ := json.Marshal(result)
			return agent.ToolResult{Content: string(data)}
		}
	}

	// Fallback to text search
	entries, err := t.db.SearchMemory(query, category, namespace, includeResolved, limit)
	if err != nil {
		return agent.ToolResult{Content: "failed to search memory: " + err.Error(), IsError: true}
	}

	result := map[string]any{
		"count":      len(entries),
		"searchType": "text",
		"entries":    entries,
	}
	data, _ := json.Marshal(result)
	return agent.ToolResult{Content: string(data)}
}
