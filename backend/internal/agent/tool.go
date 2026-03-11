package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/atroo/rancher-ai-assistant/backend/internal/vfs"
)

// ToolDefinition describes a tool for the LLM provider.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"input_schema"`
}

// ToolContext is passed to a tool during execution.
type ToolContext struct {
	Ctx    context.Context
	VFS    *vfs.Store
	Params map[string]any
}

// ToolResult is returned by a tool after execution.
type ToolResult struct {
	Content string
	IsError bool
}

// Tool is the interface every tool must implement.
type Tool interface {
	// Definition returns the tool's schema for the LLM.
	Definition() ToolDefinition

	// Execute runs the tool with the given context and parameters.
	Execute(tc ToolContext) ToolResult
}

// ToolRegistry holds registered tools and provides lookup.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // preserves registration order
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry. Panics if a tool with the same name
// is already registered (programming error, not a runtime condition).
func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Definition().Name
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tool already registered: %s", name))
	}

	r.tools[name] = t
	r.order = append(r.order, name)
}

// Get returns a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all tool definitions in registration order.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
