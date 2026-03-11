package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter wraps an http.ResponseWriter to emit Server-Sent Events
// following the Vercel AI SDK UI Message Stream Protocol v1.
//
// Wire format: data: {json}\n\n
// Termination: data: [DONE]\n\n
// Required header: X-Vercel-AI-UI-Message-Stream: v1
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter sets the required SSE headers and returns a writer.
// Returns an error if the ResponseWriter does not support flushing.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Vercel-AI-UI-Message-Stream", "v1")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// Send writes a single SSE event as data: {json}\n\n and flushes.
func (s *SSEWriter) Send(event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE event: %w", err)
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// Done writes the termination signal and flushes.
func (s *SSEWriter) Done() error {
	_, err := fmt.Fprint(s.w, "data: [DONE]\n\n")
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// --- Vercel AI SDK UI Message Stream Protocol v1 event types ---

type StartEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	SessionID string `json:"sessionId,omitempty"`
}

type StartStepEvent struct {
	Type string `json:"type"`
}

type FinishStepEvent struct {
	Type string `json:"type"`
}

type TextStartEvent struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type TextDeltaEvent struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

type TextEndEvent struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ToolInputStartEvent struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
}

type ToolInputAvailableEvent struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Input      any    `json:"input"`
}

type ToolOutputAvailableEvent struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	Output     any    `json:"output"`
}

type ToolOutputErrorEvent struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ErrorText  string `json:"errorText"`
}

type ErrorEvent struct {
	Type      string `json:"type"`
	ErrorText string `json:"errorText"`
}

type FinishEvent struct {
	Type         string `json:"type"`
	FinishReason string `json:"finishReason"`
}
