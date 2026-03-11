package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubernetesClient struct {
	clientset *kubernetes.Clientset
}

// NewKubernetesClient creates a client using the pod's ServiceAccount.
// This SA must be configured with read-only RBAC (no write, no secrets).
func NewKubernetesClient() (*KubernetesClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &KubernetesClient{clientset: clientset}, nil
}

// GetPodLogs fetches recent log lines from a pod container.
func (k *KubernetesClient) GetPodLogs(ctx context.Context, params map[string]interface{}) (string, error) {
	namespace := stringParam(params, "namespace")
	pod := stringParam(params, "pod")
	if namespace == "" || pod == "" {
		return "", fmt.Errorf("'namespace' and 'pod' are required")
	}

	opts := &corev1.PodLogOptions{}

	if container := stringParam(params, "container"); container != "" {
		opts.Container = container
	}

	tailLines := int64(100)
	if tl, ok := params["tailLines"].(float64); ok && tl > 0 {
		tailLines = int64(tl)
	}
	if tailLines > 500 {
		tailLines = 500 // cap to avoid excessive token usage
	}
	opts.TailLines = &tailLines

	if prev, ok := params["previous"].(bool); ok && prev {
		opts.Previous = true
	}

	stream, err := k.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer stream.Close()

	// Limit log output to 64KB
	data, err := io.ReadAll(io.LimitReader(stream, 64*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(data), nil
}

// GetEvents lists Kubernetes events, optionally filtered by involved object.
func (k *KubernetesClient) GetEvents(ctx context.Context, params map[string]interface{}) (string, error) {
	namespace := stringParam(params, "namespace")
	if namespace == "" {
		return "", fmt.Errorf("'namespace' is required")
	}

	opts := metav1.ListOptions{}

	// Build field selector for filtering
	var selectors []string
	if name := stringParam(params, "involvedObjectName"); name != "" {
		selectors = append(selectors, fmt.Sprintf("involvedObject.name=%s", name))
	}
	if kind := stringParam(params, "involvedObjectKind"); kind != "" {
		selectors = append(selectors, fmt.Sprintf("involvedObject.kind=%s", kind))
	}
	if len(selectors) > 0 {
		opts.FieldSelector = strings.Join(selectors, ",")
	}

	limit := int64(50)
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int64(l)
	}
	opts.Limit = limit

	events, err := k.clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("failed to list events: %w", err)
	}

	// Extract relevant fields to reduce token usage
	type compactEvent struct {
		Type           string `json:"type"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
		InvolvedObject string `json:"involvedObject"`
		Count          int32  `json:"count"`
		LastTimestamp   string `json:"lastTimestamp"`
	}

	result := make([]compactEvent, 0, len(events.Items))
	for _, e := range events.Items {
		result = append(result, compactEvent{
			Type:           e.Type,
			Reason:         e.Reason,
			Message:        e.Message,
			InvolvedObject: fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Count:          e.Count,
			LastTimestamp:   e.LastTimestamp.Format("2006-01-02T15:04:05Z"),
		})
	}

	data, _ := json.Marshal(result)
	return string(data), nil
}

// GetResourceStatus fetches the status of a Kubernetes resource.
func (k *KubernetesClient) GetResourceStatus(ctx context.Context, params map[string]interface{}) (string, error) {
	kind := stringParam(params, "kind")
	name := stringParam(params, "name")
	namespace := stringParam(params, "namespace")

	if kind == "" || name == "" {
		return "", fmt.Errorf("'kind' and 'name' are required")
	}

	var obj interface{}
	var err error

	switch strings.ToLower(kind) {
	case "pod":
		obj, err = k.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	case "deployment":
		obj, err = k.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	case "statefulset":
		obj, err = k.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	case "daemonset":
		obj, err = k.clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	case "service":
		obj, err = k.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	case "node":
		obj, err = k.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	case "job":
		obj, err = k.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	case "cronjob":
		obj, err = k.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	case "ingress":
		obj, err = k.clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	case "persistentvolumeclaim", "pvc":
		obj, err = k.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	case "horizontalpodautoscaler", "hpa":
		obj, err = k.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		return "", fmt.Errorf("unsupported resource kind: %s", kind)
	}

	if err != nil {
		return "", fmt.Errorf("failed to get %s/%s: %w", kind, name, err)
	}

	data, _ := json.Marshal(obj)

	// Truncate very large responses
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}

	return string(data), nil
}

// ListWorkloads lists Deployments, StatefulSets, and DaemonSets.
func (k *KubernetesClient) ListWorkloads(ctx context.Context, params map[string]interface{}) (string, error) {
	namespace := stringParam(params, "namespace")
	kindFilter := strings.ToLower(stringParam(params, "kind"))

	type workloadSummary struct {
		Kind              string `json:"kind"`
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		ReadyReplicas     int32  `json:"readyReplicas"`
		DesiredReplicas   int32  `json:"desiredReplicas"`
		UpdatedReplicas   int32  `json:"updatedReplicas"`
		AvailableReplicas int32  `json:"availableReplicas"`
	}

	var results []workloadSummary

	if kindFilter == "" || kindFilter == "deployment" {
		deps, err := k.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list deployments: %w", err)
		}
		for _, d := range deps.Items {
			results = append(results, workloadSummary{
				Kind:              "Deployment",
				Name:              d.Name,
				Namespace:         d.Namespace,
				ReadyReplicas:     d.Status.ReadyReplicas,
				DesiredReplicas:   *d.Spec.Replicas,
				UpdatedReplicas:   d.Status.UpdatedReplicas,
				AvailableReplicas: d.Status.AvailableReplicas,
			})
		}
	}

	if kindFilter == "" || kindFilter == "statefulset" {
		stss, err := k.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list statefulsets: %w", err)
		}
		for _, s := range stss.Items {
			results = append(results, workloadSummary{
				Kind:              "StatefulSet",
				Name:              s.Name,
				Namespace:         s.Namespace,
				ReadyReplicas:     s.Status.ReadyReplicas,
				DesiredReplicas:   *s.Spec.Replicas,
				UpdatedReplicas:   s.Status.UpdatedReplicas,
				AvailableReplicas: s.Status.AvailableReplicas,
			})
		}
	}

	if kindFilter == "" || kindFilter == "daemonset" {
		dss, err := k.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list daemonsets: %w", err)
		}
		for _, d := range dss.Items {
			results = append(results, workloadSummary{
				Kind:              "DaemonSet",
				Name:              d.Name,
				Namespace:         d.Namespace,
				ReadyReplicas:     d.Status.NumberReady,
				DesiredReplicas:   d.Status.DesiredNumberScheduled,
				UpdatedReplicas:   d.Status.UpdatedNumberScheduled,
				AvailableReplicas: d.Status.NumberAvailable,
			})
		}
	}

	data, _ := json.Marshal(results)
	return string(data), nil
}

func stringParam(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}
