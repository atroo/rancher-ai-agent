package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Provider implements agent.Provider using the Anthropic SDK.
type Provider struct {
	client sdk.Client
	model  string
}

// New creates an Anthropic provider.
func New(apiKey, model string) *Provider {
	client := sdk.NewClient(option.WithAPIKey(apiKey))
	return &Provider{
		client: client,
		model:  model,
	}
}

// CreateMessage sends a synchronous (non-streaming) message to Anthropic.
func (p *Provider) CreateMessage(ctx context.Context, req agent.ProviderRequest) (*agent.ProviderResponse, error) {
	params := p.buildParams(req)

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	return &agent.ProviderResponse{
		Content:      fromContentBlocks(resp.Content),
		StopReason:   fromStopReason(resp.StopReason),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}, nil
}

// StreamMessage sends a streaming message to Anthropic.
// Returns a channel of stream events and begins assembling the full response.
// The channel is closed when the stream ends.
func (p *Provider) StreamMessage(ctx context.Context, req agent.ProviderRequest) (<-chan agent.StreamEvent, *agent.ProviderResponse, error) {
	params := p.buildParams(req)

	stream := p.client.Messages.NewStreaming(ctx, params)

	events := make(chan agent.StreamEvent, 64)
	assembled := &agent.ProviderResponse{}

	go func() {
		defer close(events)

		// Accumulators for building the final response
		var contentBlocks []agent.ContentBlock
		var currentToolJSON string
		var currentToolID string
		var currentToolName string
		var currentText string
		var currentIndex int = -1

		for stream.Next() {
			evt := stream.Current()

			switch evt.Type {
			case "content_block_start":
				currentIndex++
				if evt.ContentBlock.Type == "text" {
					currentText = ""
					events <- agent.StreamEvent{
						Type:  agent.StreamEventContentStart,
						Index: currentIndex,
					}
				} else if evt.ContentBlock.Type == "tool_use" {
					currentToolID = evt.ContentBlock.ID
					currentToolName = evt.ContentBlock.Name
					currentToolJSON = ""
					events <- agent.StreamEvent{
						Type:       agent.StreamEventToolUseStart,
						Index:      currentIndex,
						ToolCallID: currentToolID,
						ToolName:   currentToolName,
					}
				}

			case "content_block_delta":
				if evt.Delta.Type == "text_delta" {
					currentText += evt.Delta.Text
					events <- agent.StreamEvent{
						Type:  agent.StreamEventTextDelta,
						Index: currentIndex,
						Text:  evt.Delta.Text,
					}
				} else if evt.Delta.Type == "input_json_delta" {
					currentToolJSON += evt.Delta.PartialJSON
					events <- agent.StreamEvent{
						Type:        agent.StreamEventToolUseDelta,
						Index:       currentIndex,
						ToolCallID:  currentToolID,
						PartialJSON: evt.Delta.PartialJSON,
					}
				}

			case "content_block_stop":
				events <- agent.StreamEvent{
					Type:  agent.StreamEventContentEnd,
					Index: currentIndex,
				}

				// Assemble the completed block
				if currentToolID != "" {
					contentBlocks = append(contentBlocks, agent.ContentBlock{
						Type:       "tool_use",
						ToolCallID: currentToolID,
						ToolName:   currentToolName,
						Input:      json.RawMessage(currentToolJSON),
					})
					currentToolID = ""
					currentToolName = ""
					currentToolJSON = ""
				} else {
					contentBlocks = append(contentBlocks, agent.ContentBlock{
						Type: "text",
						Text: currentText,
					})
					currentText = ""
				}

			case "message_delta":
				assembled.StopReason = fromStopReason(evt.Delta.StopReason)
				assembled.OutputTokens = int(evt.Usage.OutputTokens)

			case "message_start":
				if evt.Message.Usage.InputTokens > 0 {
					assembled.InputTokens = int(evt.Message.Usage.InputTokens)
				}
			}
		}

		if err := stream.Err(); err != nil {
			events <- agent.StreamEvent{
				Type: agent.StreamEventMessageEnd,
				Text: fmt.Sprintf("stream error: %s", err),
			}
		}

		assembled.Content = contentBlocks

		events <- agent.StreamEvent{
			Type:         agent.StreamEventMessageEnd,
			StopReason:   assembled.StopReason,
			InputTokens:  assembled.InputTokens,
			OutputTokens: assembled.OutputTokens,
		}
	}()

	return events, assembled, nil
}

func (p *Provider) buildParams(req agent.ProviderRequest) sdk.MessageNewParams {
	params := sdk.MessageNewParams{
		Model:     sdk.Model(p.model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  toMessages(req.Messages),
	}

	if req.SystemPrompt != "" {
		params.System = []sdk.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	if len(req.Tools) > 0 {
		params.Tools = toTools(req.Tools)
	}

	return params
}
