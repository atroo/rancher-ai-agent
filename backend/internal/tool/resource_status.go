package tool

import (
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type ResourceStatusTool struct {
	client *datasource.KubernetesClient
}

func NewResourceStatusTool(client *datasource.KubernetesClient) *ResourceStatusTool {
	return &ResourceStatusTool{client: client}
}

func (t *ResourceStatusTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "get_resource_status",
		Description: "Get the status and conditions of a Kubernetes resource. Large responses are stored in the VFS; small ones are returned directly.",
		Parameters: schema(map[string]any{
			"kind":      map[string]any{"type": "string", "description": "Resource kind: Pod, Deployment, StatefulSet, DaemonSet, Service, Node, Job, CronJob, Ingress, PersistentVolumeClaim, HorizontalPodAutoscaler"},
			"namespace": map[string]any{"type": "string", "description": "Resource namespace (omit for cluster-scoped resources like Node)"},
			"name":      map[string]any{"type": "string", "description": "Resource name"},
		}, []string{"kind", "name"}),
	}
}

func (t *ResourceStatusTool) Execute(tc agent.ToolContext) agent.ToolResult {
	raw, err := t.client.GetResourceStatus(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}

	// Small responses go directly to LLM; large ones go to VFS
	if len(raw) < 4096 {
		return agent.ToolResult{Content: raw}
	}

	desc := fmt.Sprintf("Status: %s/%s", stringParam(tc.Params, "kind"), stringParam(tc.Params, "name"))
	f := tc.VFS.Write("k8s_status", desc, raw)
	return agent.ToolResult{
		Content: fmt.Sprintf("Resource status stored in %s (%d bytes). Use read_file or query_json to inspect.", f.Path, f.Size),
	}
}
