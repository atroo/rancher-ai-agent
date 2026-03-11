package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type ListWorkloadsTool struct {
	client *datasource.KubernetesClient
}

func NewListWorkloadsTool(client *datasource.KubernetesClient) *ListWorkloadsTool {
	return &ListWorkloadsTool{client: client}
}

func (t *ListWorkloadsTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "list_workloads",
		Description: "List workloads (Deployments, StatefulSets, DaemonSets) in a namespace with their replica counts and conditions.",
		Parameters: schema(map[string]any{
			"namespace": map[string]any{"type": "string", "description": "Namespace to list workloads in. Omit to list across all namespaces."},
			"kind":      map[string]any{"type": "string", "description": "Filter by kind: Deployment, StatefulSet, DaemonSet. Omit to list all types."},
		}, nil),
	}
}

func (t *ListWorkloadsTool) Execute(tc agent.ToolContext) agent.ToolResult {
	result, err := t.client.ListWorkloads(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}
	return agent.ToolResult{Content: result}
}
