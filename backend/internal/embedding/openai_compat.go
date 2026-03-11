package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAICompatProvider works with any OpenAI-compatible embeddings API:
// OpenAI, Voyage AI, Azure OpenAI, Ollama, vLLM, etc.
type OpenAICompatProvider struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

// OpenAICompatConfig configures the embedding provider.
type OpenAICompatConfig struct {
	// BaseURL is the API base (e.g., "https://api.openai.com/v1",
	// "https://api.voyageai.com/v1", "http://localhost:11434/v1")
	BaseURL string

	// APIKey for authentication
	APIKey string

	// Model name (e.g., "text-embedding-3-small", "voyage-3-lite", "nomic-embed-text")
	Model string

	// Dimensions of the output vectors
	Dimensions int
}

func NewOpenAICompatProvider(cfg OpenAICompatConfig) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenAICompatProvider) Dimensions() int {
	return p.dimensions
}

func (p *OpenAICompatProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

func (p *OpenAICompatProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := embeddingRequest{
		Model: p.model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Sort by index to ensure order matches input
	embeddings := make([][]float32, len(texts))
	for _, item := range result.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}
