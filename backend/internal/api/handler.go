package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
)

// ChatRequest is the JSON payload for the SSE chat endpoint.
type ChatRequest struct {
	Message   string       `json:"message"`
	SessionID string       `json:"sessionId,omitempty"`
	Context   *ChatContext `json:"context,omitempty"`
}

// ChatContext carries optional Rancher UI context.
type ChatContext struct {
	ClusterID    string `json:"clusterId,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
}

type Handler struct {
	agent    *agent.Agent
	msgCount atomic.Int64
}

func NewHandler(a *agent.Agent) *Handler {
	return &Handler{agent: a}
}

// RegisterRoutes mounts all HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health", h.handleHealth)
	mux.HandleFunc("POST /api/v1/chat", h.handleChat)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleChat streams the assistant response using the Vercel AI SDK
// UI Message Stream Protocol v1 (Server-Sent Events).
func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
		return
	}

	slog.Info("chat request",
		"user", user.Username,
		"session", req.SessionID,
		"message_length", len(req.Message),
	)

	// Set up SSE writer
	sse, err := NewSSEWriter(w)
	if err != nil {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", h.msgCount.Add(1))
	}
	msgID := fmt.Sprintf("msg_%d", h.msgCount.Add(1))

	// Send start event with session ID so the client can continue the conversation
	sse.Send(StartEvent{Type: "start", MessageID: msgID, SessionID: sessionID})

	var convCtx agent.ConversationContext
	if req.Context != nil {
		convCtx = agent.ConversationContext{
			ClusterID:    req.Context.ClusterID,
			Namespace:    req.Context.Namespace,
			ResourceType: req.Context.ResourceType,
			ResourceName: req.Context.ResourceName,
		}
	}

	// Run the agent — consume events from the channel and forward as SSE
	events := h.agent.Run(r.Context(), agent.RunOptions{
		SessionID:   sessionID,
		UserMessage: req.Message,
		ConvCtx:     convCtx,
	})

	for event := range events {
		switch event.Type {
		case agent.EventStepStart:
			sse.Send(StartStepEvent{Type: "start-step"})

		case agent.EventStepFinish:
			sse.Send(FinishStepEvent{Type: "finish-step"})

		case agent.EventTextDelta:
			d := event.Data.(agent.TextDeltaData)
			sse.Send(TextStartEvent{Type: "text-start", ID: d.ID})
			sse.Send(TextDeltaEvent{Type: "text-delta", ID: d.ID, Delta: d.Delta})

		case agent.EventTextDone:
			d := event.Data.(agent.TextDoneData)
			sse.Send(TextEndEvent{Type: "text-end", ID: d.ID})

		case agent.EventToolCallStart:
			d := event.Data.(agent.ToolCallStartData)
			sse.Send(ToolInputStartEvent{
				Type:       "tool-input-start",
				ToolCallID: d.ToolCallID,
				ToolName:   d.ToolName,
			})
			sse.Send(ToolInputAvailableEvent{
				Type:       "tool-input-available",
				ToolCallID: d.ToolCallID,
				ToolName:   d.ToolName,
				Input:      d.Input,
			})

		case agent.EventToolCallResult:
			d := event.Data.(agent.ToolCallResultData)
			sse.Send(ToolOutputAvailableEvent{
				Type:       "tool-output-available",
				ToolCallID: d.ToolCallID,
				Output:     d.Output,
			})

		case agent.EventToolCallError:
			d := event.Data.(agent.ToolCallErrorData)
			sse.Send(ToolOutputErrorEvent{
				Type:       "tool-output-error",
				ToolCallID: d.ToolCallID,
				ErrorText:  d.Error,
			})

		case agent.EventError:
			d := event.Data.(agent.ErrorData)
			sse.Send(ErrorEvent{Type: "error", ErrorText: d.Error})

		case agent.EventFinish:
			d := event.Data.(agent.FinishData)
			sse.Send(FinishEvent{Type: "finish", FinishReason: d.Reason})
		}
	}

	sse.Done()
}
