package tool

import (
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
)

type PodLogsTool struct {
	client *datasource.KubernetesClient
}

func NewPodLogsTool(client *datasource.KubernetesClient) *PodLogsTool {
	return &PodLogsTool{client: client}
}

func (t *PodLogsTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "get_pod_logs",
		Description: "Fetch recent logs from a pod's container. Logs are stored in the virtual filesystem; you receive a summary (line count, last few lines). Use read_file or search_in_file to inspect the logs.",
		Parameters: schema(map[string]any{
			"namespace": map[string]any{"type": "string", "description": "Pod namespace"},
			"pod":       map[string]any{"type": "string", "description": "Pod name"},
			"container": map[string]any{"type": "string", "description": "Container name. Omit if the pod has a single container."},
			"tailLines": map[string]any{"type": "integer", "description": "Number of lines from the end. Default 100."},
			"previous":  map[string]any{"type": "boolean", "description": "If true, return logs from the previous terminated container (useful for crash loops)."},
		}, []string{"namespace", "pod"}),
	}
}

func (t *PodLogsTool) Execute(tc agent.ToolContext) agent.ToolResult {
	raw, err := t.client.GetPodLogs(tc.Ctx, tc.Params)
	if err != nil {
		return agent.ToolResult{Content: err.Error(), IsError: true}
	}

	desc := fmt.Sprintf("Logs: %s/%s", stringParam(tc.Params, "namespace"), stringParam(tc.Params, "pod"))
	f := tc.VFS.Write("pod_logs", desc, raw)
	return agent.ToolResult{Content: SummarizePodLogs(raw, f.Path)}
}
