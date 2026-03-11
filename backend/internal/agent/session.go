package agent

import (
	"sync"
	"time"

	"github.com/atroo/rancher-ai-assistant/backend/internal/vfs"
)

// Session holds conversation state: message history, VFS, and token tracking.
type Session struct {
	ID        string
	VFS       *vfs.Store
	Budget    *TokenBudget
	CreatedAt time.Time
	LastUsed  time.Time

	mu       sync.Mutex
	messages []Message
}

// NewSession creates a new session with an empty VFS and token budget.
func NewSession(id string, tokenBudget int) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		VFS:       vfs.New(),
		Budget:    NewTokenBudget(tokenBudget),
		CreatedAt: now,
		LastUsed:  now,
	}
}

// AppendUser adds a user message to the conversation history.
func (s *Session) AppendUser(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, Message{
		Role: RoleUser,
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
	})
	s.LastUsed = time.Now()
}

// AppendAssistant adds an assistant response to the history.
func (s *Session) AppendAssistant(blocks []ContentBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, Message{
		Role:    RoleAssistant,
		Content: blocks,
	})
	s.LastUsed = time.Now()
}

// AppendToolResults adds tool results as a user message (per Anthropic convention).
func (s *Session) AppendToolResults(results []ContentBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, Message{
		Role:    RoleUser,
		Content: results,
	})
	s.LastUsed = time.Now()
}

// History returns a copy of the full message history.
func (s *Session) History() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// TrackTokens records token usage from a provider response.
func (s *Session) TrackTokens(input, output int) error {
	return s.Budget.Record(input + output)
}

// MessageCount returns the number of messages in history.
func (s *Session) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}
