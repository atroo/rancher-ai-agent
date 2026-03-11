package tool

import (
	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type K8sEventsTool struct {
	client *datasource.KubernetesClient
}

func NewK8sEventsTool(client *datasource.KubernetesClient) *K8sEventsTool {
	return &K8sEventsTool{client: client}
}

func (t *K8sEventsTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "get_k8s_events",
		Description: "List recent Kubernetes events for a resource or namespace. Events are compact enough to return directly.",
		Parameters: schema(map[string]any{
			"namespace":          map[string]any{"type": "string", "description": "Namespace to search events in"},
			"involvedObjectName": map[string]any{"type": "string", "description": "Filter events by the involved object's name (e.g., pod name)"},
			"involvedObjectKind": map[string]any{"type": "string", "description": "Filter events by kind (e.g., Pod, Deployment, Node)"},
			"limit":              map[string]any{"type": "integer", "description": "Max events to return. Default 50."},
		}, []string{"namespace"}),
	}
}

func (t *K8sEventsTool) Execute(tc agent.ToolContext) agent.ToolResult {
	result, err := t.client.GetEvents(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}
	return agent.ToolResult{Content: result}
}
