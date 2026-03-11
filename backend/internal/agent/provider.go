package agent

import (
	"context"
	"encoding/json"
)

// Role represents a message participant.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock is a typed block within a message (text, tool_use, tool_result).
type ContentBlock struct {
	Type       string          `json:"type"`                  // "text", "tool_use", "tool_result"
	Text       string          `json:"text,omitempty"`        // for "text"
	ToolCallID string          `json:"tool_call_id,omitempty"` // for "tool_use" and "tool_result"
	ToolName   string          `json:"tool_name,omitempty"`   // for "tool_use"
	Input      json.RawMessage `json:"input,omitempty"`       // for "tool_use"
	Content    string          `json:"content,omitempty"`     // for "tool_result"
	IsError    bool            `json:"is_error,omitempty"`    // for "tool_result"
}

// ProviderRequest is sent to the LLM provider.
type ProviderRequest struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
}

// ProviderResponse is the assembled result from the LLM provider.
type ProviderResponse struct {
	Content      []ContentBlock
	StopReason   string // "end_turn", "tool_use", "max_tokens"
	InputTokens  int
	OutputTokens int
}

// StreamEvent is emitted by the provider during streaming.
type StreamEvent struct {
	Type         string // see StreamEvent* constants
	Index        int
	Text         string          // for StreamEventTextDelta
	ToolCallID   string          // for StreamEventToolUseStart
	ToolName     string          // for StreamEventToolUseStart
	PartialJSON  string          // for StreamEventToolUseDelta
	StopReason   string          // for StreamEventMessageEnd
	InputTokens  int             // for StreamEventMessageEnd
	OutputTokens int             // for StreamEventMessageEnd
}

const (
	StreamEventContentStart  = "content_start"
	StreamEventTextDelta     = "text_delta"
	StreamEventToolUseStart  = "tool_use_start"
	StreamEventToolUseDelta  = "tool_use_delta"
	StreamEventContentEnd    = "content_end"
	StreamEventMessageEnd    = "message_end"
)

// Provider abstracts an LLM backend (Anthropic, OpenAI, Ollama, etc.).
type Provider interface {
	// CreateMessage sends a synchronous request and returns the full response.
	CreateMessage(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)

	// StreamMessage sends a streaming request. Events are sent to the returned channel,
	// which is closed when the stream completes. The caller should range over the channel.
	// On error, the channel receives a final event and is then closed; the error is
	// also returned in ProviderResponse from the assembled result.
	StreamMessage(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, *ProviderResponse, error)
}
