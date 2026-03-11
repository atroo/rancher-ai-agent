package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Agent orchestrates the LLM tool-use loop with streaming, parallel tool
// execution, conversation memory, and token budget tracking.
type Agent struct {
	provider   Provider
	registry   *ToolRegistry
	sessions   SessionStore
	config     Config
	middleware *MiddlewareChain
	promptFn   func(ConversationContext) string
}

// RunOptions configures a single agent turn.
type RunOptions struct {
	SessionID   string
	UserMessage string
	ConvCtx     ConversationContext
}

// Option is a functional option for configuring the agent.
type Option func(*Agent)

func WithConfig(cfg Config) Option              { return func(a *Agent) { a.config = cfg } }
func WithSessionStore(s SessionStore) Option     { return func(a *Agent) { a.sessions = s } }
func WithMiddleware(mws ...ToolMiddleware) Option {
	return func(a *Agent) { a.middleware = NewMiddlewareChain(mws...) }
}
func WithSystemPrompt(fn func(ConversationContext) string) Option {
	return func(a *Agent) { a.promptFn = fn }
}

// New creates an agent. The provider should already be wrapped with retry if desired.
func New(provider Provider, registry *ToolRegistry, opts ...Option) *Agent {
	a := &Agent{
		provider:   provider,
		registry:   registry,
		config:     DefaultConfig(),
		middleware: NewMiddlewareChain(),
		promptFn:   BuildSystemPrompt,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Default session store if none provided
	if a.sessions == nil {
		a.sessions = NewMemorySessionStore(a.config.SessionTTL)
	}

	return a
}

// Run executes a user turn, streaming events to the returned channel.
// The channel is closed when the turn is complete.
func (a *Agent) Run(ctx context.Context, opts RunOptions) <-chan Event {
	events := make(chan Event, 64)

	go func() {
		defer close(events)
		a.run(ctx, opts, events)
	}()

	return events
}

func (a *Agent) run(ctx context.Context, opts RunOptions, events chan<- Event) {
	session := a.sessions.GetOrCreate(opts.SessionID, a.config.MaxTokensBudget)

	// Persist session state when the turn completes
	defer a.sessions.Persist(session)

	// Append user message to conversation history
	session.AppendUser(opts.UserMessage)

	systemPrompt := a.promptFn(opts.ConvCtx)
	toolDefs := a.registry.Definitions()
	var textBlockCount int

	for round := range a.config.MaxToolRounds {
		slog.Info("agent round", "round", round, "session", opts.SessionID, "messages", session.MessageCount())

		emit(events, EventStepStart, nil)

		// Call the provider (streaming)
		streamEvents, assembled, err := a.provider.StreamMessage(ctx, ProviderRequest{
			SystemPrompt: systemPrompt,
			Messages:     session.History(),
			Tools:        toolDefs,
			MaxTokens:    a.config.MaxTokensPerTurn,
		})
		if err != nil {
			emit(events, EventError, ErrorData{Error: fmt.Sprintf("provider error: %s", err)})
			emit(events, EventFinish, FinishData{Reason: "error"})
			return
		}

		// Process streaming events — forward text deltas immediately
		var currentTextID string
		var currentTextContent string

		for se := range streamEvents {
			switch se.Type {
			case StreamEventTextDelta:
				if currentTextID == "" {
					textBlockCount++
					currentTextID = fmt.Sprintf("txt_%d", textBlockCount)
				}
				currentTextContent += se.Text
				emit(events, EventTextDelta, TextDeltaData{ID: currentTextID, Delta: se.Text})

			case StreamEventContentEnd:
				if currentTextID != "" {
					emit(events, EventTextDone, TextDoneData{ID: currentTextID, FullText: currentTextContent})
					currentTextID = ""
					currentTextContent = ""
				}

			case StreamEventToolUseStart:
				// Tool starts are informational during streaming; actual execution happens after

			case StreamEventMessageEnd:
				// Token tracking
				if err := session.TrackTokens(se.InputTokens, se.OutputTokens); err != nil {
					emit(events, EventError, ErrorData{Error: "token budget exceeded"})
					emit(events, EventFinish, FinishData{
						Reason:      "token_budget_exceeded",
						TotalInput:  session.Budget.Used(),
						TotalOutput: 0,
					})
					return
				}
			}
		}

		// Assembled response is now complete
		if assembled == nil {
			emit(events, EventError, ErrorData{Error: "no response from provider"})
			emit(events, EventFinish, FinishData{Reason: "error"})
			return
		}

		// Save assistant response to session history
		session.AppendAssistant(assembled.Content)

		// Check for tool use blocks
		var toolUseBlocks []ContentBlock
		for _, block := range assembled.Content {
			if block.Type == "tool_use" {
				toolUseBlocks = append(toolUseBlocks, block)
			}
		}

		emit(events, EventStepFinish, nil)

		if len(toolUseBlocks) == 0 {
			// No tool calls — final answer was streamed
			in, _ := session.Budget.Used(), 0
			emit(events, EventFinish, FinishData{Reason: "stop", TotalInput: in})
			return
		}

		// Execute tools (potentially in parallel)
		toolResults := a.executeTools(ctx, session, toolUseBlocks, events)

		// Append tool results to session history
		session.AppendToolResults(toolResults)
	}

	emit(events, EventError, ErrorData{Error: fmt.Sprintf("exceeded maximum tool rounds (%d)", a.config.MaxToolRounds)})
	emit(events, EventFinish, FinishData{Reason: "max_rounds"})
}

// executeTools runs tools concurrently up to ToolConcurrency, emitting events for each.
func (a *Agent) executeTools(ctx context.Context, session *Session, toolCalls []ContentBlock, events chan<- Event) []ContentBlock {
	results := make([]ContentBlock, len(toolCalls))
	sem := make(chan struct{}, a.config.ToolConcurrency)
	var wg sync.WaitGroup

	for i, call := range toolCalls {
		wg.Add(1)
		go func(i int, call ContentBlock) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tool, ok := a.registry.Get(call.ToolName)
			if !ok {
				errMsg := fmt.Sprintf("unknown tool: %s", call.ToolName)
				emit(events, EventToolCallError, ToolCallErrorData{
					ToolCallID: call.ToolCallID,
					Error:      errMsg,
				})
				results[i] = ContentBlock{
					Type:       "tool_result",
					ToolCallID: call.ToolCallID,
					Content:    errMsg,
					IsError:    true,
				}
				return
			}

			// Parse input
			var params map[string]any
			if err := json.Unmarshal(call.Input, &params); err != nil {
				params = make(map[string]any)
			}

			// Emit start event
			emit(events, EventToolCallStart, ToolCallStartData{
				ToolCallID: call.ToolCallID,
				ToolName:   call.ToolName,
				Input:      params,
			})

			// Execute through middleware chain
			tc := ToolContext{
				Ctx:    ctx,
				VFS:    session.VFS,
				Params: params,
			}
			result := a.middleware.Execute(tc, tool)

			// Emit result event
			if result.IsError {
				emit(events, EventToolCallError, ToolCallErrorData{
					ToolCallID: call.ToolCallID,
					Error:      result.Content,
				})
			} else {
				// Try to parse as JSON for structured output
				var structured any
				if err := json.Unmarshal([]byte(result.Content), &structured); err != nil {
					structured = result.Content
				}
				emit(events, EventToolCallResult, ToolCallResultData{
					ToolCallID: call.ToolCallID,
					Output:     structured,
				})
			}

			results[i] = ContentBlock{
				Type:       "tool_result",
				ToolCallID: call.ToolCallID,
				Content:    result.Content,
				IsError:    result.IsError,
			}
		}(i, call)
	}

	wg.Wait()
	return results
}

func emit(ch chan<- Event, typ string, data any) {
	ch <- Event{Type: typ, Data: data}
}
