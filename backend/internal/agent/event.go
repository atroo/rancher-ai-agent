package agent

// Event is emitted by the agent loop for callers to consume (SSE handler, logging, etc.).
type Event struct {
	Type string
	Data any
}

// Event type constants.
const (
	EventStepStart     = "step_start"
	EventStepFinish    = "step_finish"
	EventTextDelta     = "text_delta"
	EventTextDone      = "text_done"
	EventToolCallStart = "tool_call_start"
	EventToolCallResult = "tool_call_result"
	EventToolCallError = "tool_call_error"
	EventError         = "error"
	EventFinish        = "finish"
)

// TextDeltaData is the payload for EventTextDelta.
type TextDeltaData struct {
	ID    string
	Delta string
}

// TextDoneData is the payload for EventTextDone.
type TextDoneData struct {
	ID       string
	FullText string
}

// ToolCallStartData is the payload for EventToolCallStart.
type ToolCallStartData struct {
	ToolCallID string
	ToolName   string
	Input      any
}

// ToolCallResultData is the payload for EventToolCallResult.
type ToolCallResultData struct {
	ToolCallID string
	Output     any
}

// ToolCallErrorData is the payload for EventToolCallError.
type ToolCallErrorData struct {
	ToolCallID string
	Error      string
}

// ErrorData is the payload for EventError.
type ErrorData struct {
	Error string
}

// FinishData is the payload for EventFinish.
type FinishData struct {
	Reason      string
	TotalInput  int
	TotalOutput int
}
