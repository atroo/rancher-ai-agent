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

type TempoClient struct {
	baseURL string
	http    *http.Client
}

func NewTempoClient(baseURL string) *TempoClient {
	return &TempoClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search executes a TraceQL query against Tempo.
func (t *TempoClient) Search(ctx context.Context, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("'query' parameter is required")
	}

	u, _ := url.Parse(t.baseURL + "/api/search")
	q := u.Query()
	q.Set("q", query)

	if limit, ok := params["limit"].(float64); ok {
		q.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		q.Set("limit", "20")
	}
	if start, ok := params["start"].(string); ok && start != "" {
		q.Set("start", start)
	}
	if end, ok := params["end"].(string); ok && end != "" {
		q.Set("end", end)
	}

	u.RawQuery = q.Encode()

	return t.doGet(ctx, u.String())
}

// GetTrace fetches a complete trace by ID.
func (t *TempoClient) GetTrace(ctx context.Context, traceID string) (string, error) {
	if traceID == "" {
		return "", fmt.Errorf("traceId is required")
	}

	u := fmt.Sprintf("%s/api/traces/%s", t.baseURL, url.PathEscape(traceID))
	return t.doGet(ctx, u)
}

func (t *TempoClient) doGet(ctx context.Context, reqURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("tempo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("failed to read tempo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tempo returned %d: %s", resp.StatusCode, string(body))
	}

	var compact json.RawMessage
	if err := json.Unmarshal(body, &compact); err != nil {
		return string(body), nil
	}
	compacted, _ := json.Marshal(compact)
	return string(compacted), nil
}
