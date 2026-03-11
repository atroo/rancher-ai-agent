package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type PrometheusClient struct {
	baseURL string
	http    *http.Client
}

func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Query executes an instant or range PromQL query.
func (p *PrometheusClient) Query(ctx context.Context, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("'query' parameter is required")
	}

	// Decide between instant and range query
	_, hasStart := params["start"]
	_, hasEnd := params["end"]

	if hasStart && hasEnd {
		return p.rangeQuery(ctx, query, params)
	}
	return p.instantQuery(ctx, query, params)
}

func (p *PrometheusClient) instantQuery(ctx context.Context, query string, params map[string]interface{}) (string, error) {
	u, _ := url.Parse(p.baseURL + "/api/v1/query")
	q := u.Query()
	q.Set("query", query)
	if t, ok := params["time"].(string); ok && t != "" {
		q.Set("time", t)
	}
	u.RawQuery = q.Encode()

	return p.doGet(ctx, u.String())
}

func (p *PrometheusClient) rangeQuery(ctx context.Context, query string, params map[string]interface{}) (string, error) {
	u, _ := url.Parse(p.baseURL + "/api/v1/query_range")
	q := u.Query()
	q.Set("query", query)

	if start, ok := params["start"].(string); ok {
		q.Set("start", start)
	}
	if end, ok := params["end"].(string); ok {
		q.Set("end", end)
	}
	step := "60s"
	if s, ok := params["step"].(string); ok && s != "" {
		step = s
	}
	q.Set("step", step)

	u.RawQuery = q.Encode()

	return p.doGet(ctx, u.String())
}

func (p *PrometheusClient) doGet(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("prometheus request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512KB limit
	if err != nil {
		return "", fmt.Errorf("failed to read prometheus response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	// Compact the JSON to save tokens
	var compact json.RawMessage
	if err := json.Unmarshal(body, &compact); err != nil {
		return string(body), nil
	}
	compacted, _ := json.Marshal(compact)
	return string(compacted), nil
}
