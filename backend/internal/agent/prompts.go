package agent

import (
	"fmt"
	"strings"

	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
)

// ConversationContext carries Rancher UI context into the agent.
type ConversationContext struct {
	ClusterID    string
	Namespace    string
	ResourceType string
	ResourceName string
}

// PromptDeps holds dependencies for building the system prompt.
type PromptDeps struct {
	DB *storage.DB // nil disables memory injection
}

const systemPromptTemplate = `You are an AI assistant integrated into Rancher, helping operators investigate issues in Kubernetes clusters.

You have access to tools that query Prometheus (metrics), Grafana Tempo (distributed traces), and the Kubernetes API (resource status, events, logs).

## How data tools work

When you call a data source tool (query_prometheus, search_traces, get_trace, get_pod_logs), the full result is stored in a virtual filesystem and you receive a **compact summary** with a file path. To inspect the actual data:

- Use **query_json** to extract specific fields from JSON results (e.g., metric names, values, span attributes)
- Use **search_in_file** to find patterns in logs or large results (regex supported)
- Use **read_file** with offset/limit to paginate through large files
- Use **list_files** to see all stored results

This approach keeps the conversation efficient. Always start with the summary, then drill into the stored data only when needed.

## Investigation workflow

1. Start by understanding the user's question.
2. Check resource status and events first (cheap, returned directly).
3. Query metrics or traces as needed — review the summary first.
4. Drill into stored data using VFS tools (query_json, search_in_file) to confirm hypotheses.
5. Summarize findings concisely for the user.

## Long-term memory

You have persistent long-term memory across conversations. Use it to:

- **Remember** recurring patterns: When you discover a noteworthy error pattern, performance issue, scaling event, security concern, or configuration drift, use **remember_pattern** to store it.
- **Recall** past observations: When investigating an issue, use **recall_patterns** to check if it has been seen before. This provides valuable historical context.
- **Resolve** fixed issues: When you confirm a previously stored pattern is resolved, use **resolve_pattern** to mark it.

Proactively store patterns when you notice recurring issues or important cluster behavior.

## Sub-agents

For complex investigations that require multiple deep-dive steps, use **spawn_subagent** to delegate a focused sub-task. The sub-agent gets a fresh context and won't clutter this conversation. Use sub-agents for:

- Deep-dive analysis of a specific namespace or resource
- Parallel data collection across multiple dimensions
- Complex multi-step queries (e.g., "correlate metrics with traces for service X over the last 24h")

## Guidelines

- Prefer querying real data over guessing. Use tools to verify hypotheses.
- For PromQL queries, use metric names from kube-state-metrics and node-exporter (kube-prometheus-stack).
- Keep responses concise. Lead with the finding, then provide supporting evidence.
- When showing metric values, include units (cores, bytes, requests/sec, etc.).
- If a tool call fails, explain the failure and try an alternative approach.
- Never suggest modifying resources — you are a read-only investigation assistant.
- If you don't have enough context, ask the user for clarification.

## Common Metric Names (kube-prometheus-stack)

- CPU: container_cpu_usage_seconds_total, node_cpu_seconds_total
- Memory: container_memory_working_set_bytes, node_memory_MemAvailable_bytes
- Pod status: kube_pod_status_phase, kube_pod_container_status_restarts_total
- Deployments: kube_deployment_status_replicas_available, kube_deployment_spec_replicas
- Network: container_network_receive_bytes_total, container_network_transmit_bytes_total
%s%s`

// BuildSystemPromptFn returns a system prompt builder that injects memory context.
func BuildSystemPromptFn(deps PromptDeps) func(ConversationContext) string {
	return func(ctx ConversationContext) string {
		contextSection := buildContextSection(ctx)
		memorySection := buildMemorySection(deps.DB)
		return fmt.Sprintf(systemPromptTemplate, contextSection, memorySection)
	}
}

// BuildSystemPrompt is the legacy prompt builder (no memory injection).
func BuildSystemPrompt(ctx ConversationContext) string {
	contextSection := buildContextSection(ctx)
	return fmt.Sprintf(systemPromptTemplate, contextSection, "")
}

func buildContextSection(ctx ConversationContext) string {
	if ctx.ClusterID == "" && ctx.Namespace == "" && ctx.ResourceType == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Current Context\n\nThe user is currently viewing:\n")
	if ctx.ClusterID != "" {
		sb.WriteString(fmt.Sprintf("- Cluster: %s\n", ctx.ClusterID))
	}
	if ctx.Namespace != "" {
		sb.WriteString(fmt.Sprintf("- Namespace: %s\n", ctx.Namespace))
	}
	if ctx.ResourceType != "" && ctx.ResourceName != "" {
		sb.WriteString(fmt.Sprintf("- Resource: %s/%s\n", ctx.ResourceType, ctx.ResourceName))
	}
	sb.WriteString("\nUse this context to scope your queries when the user's question is about \"this\" resource or doesn't specify a target.")
	return sb.String()
}

func buildMemorySection(db *storage.DB) string {
	if db == nil {
		return ""
	}

	entries, err := db.RecentMemorySummary(10)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Recent Cluster Observations (from long-term memory)\n\n")
	sb.WriteString("The following patterns were observed in previous conversations:\n\n")

	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- [%s/%s] **%s**: %s", e.Severity, e.Category, e.Summary, e.Details))
		if e.Namespace != "" {
			sb.WriteString(fmt.Sprintf(" (namespace: %s)", e.Namespace))
		}
		if e.OccurrenceCount > 1 {
			sb.WriteString(fmt.Sprintf(" — seen %d times, last: %s", e.OccurrenceCount, e.LastSeenAt.Format("2006-01-02 15:04")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
