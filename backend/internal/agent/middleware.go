package agent

import (
	"log/slog"
	"time"
)

// ToolMiddleware wraps tool execution. Call next to proceed to the next
// middleware (or the actual tool execution at the end of the chain).
type ToolMiddleware func(tc ToolContext, tool Tool, next func(ToolContext) ToolResult) ToolResult

// MiddlewareChain applies middleware in order around a tool execution.
type MiddlewareChain struct {
	middlewares []ToolMiddleware
}

// NewMiddlewareChain creates a chain from the given middleware functions.
func NewMiddlewareChain(mws ...ToolMiddleware) *MiddlewareChain {
	return &MiddlewareChain{middlewares: mws}
}

// Execute runs the tool through the middleware chain.
func (mc *MiddlewareChain) Execute(tc ToolContext, tool Tool) ToolResult {
	if len(mc.middlewares) == 0 {
		return tool.Execute(tc)
	}

	// Build the chain from the inside out
	var build func(i int) func(ToolContext) ToolResult
	build = func(i int) func(ToolContext) ToolResult {
		if i >= len(mc.middlewares) {
			return func(tc ToolContext) ToolResult {
				return tool.Execute(tc)
			}
		}
		mw := mc.middlewares[i]
		next := build(i + 1)
		return func(tc ToolContext) ToolResult {
			return mw(tc, tool, next)
		}
	}

	return build(0)(tc)
}

// LoggingMiddleware logs tool name, duration, and success/failure.
func LoggingMiddleware(tc ToolContext, tool Tool, next func(ToolContext) ToolResult) ToolResult {
	name := tool.Definition().Name
	start := time.Now()

	result := next(tc)

	duration := time.Since(start)
	if result.IsError {
		slog.Warn("tool execution failed",
			"tool", name,
			"duration", duration,
			"error", result.Content,
		)
	} else {
		slog.Info("tool executed",
			"tool", name,
			"duration", duration,
			"result_size", len(result.Content),
		)
	}

	return result
}
