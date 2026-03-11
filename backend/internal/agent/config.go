package agent

import "time"

// Config controls agent behavior.
type Config struct {
	// MaxToolRounds is the maximum number of LLM ↔ tool round trips per turn.
	MaxToolRounds int

	// MaxTokensPerTurn is the max_tokens parameter sent to the provider per call.
	MaxTokensPerTurn int

	// MaxTokensBudget is the total token limit per session (input + output).
	// 0 means unlimited.
	MaxTokensBudget int

	// ToolConcurrency is the maximum number of tools executed in parallel
	// when the LLM requests multiple tool calls in one response.
	ToolConcurrency int

	// RetryAttempts is how many times to retry a failed provider call.
	RetryAttempts int

	// RetryBaseDelay is the base delay for exponential backoff on retries.
	RetryBaseDelay time.Duration

	// SessionTTL is how long idle sessions are kept in memory.
	SessionTTL time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxToolRounds:    10,
		MaxTokensPerTurn: 4096,
		MaxTokensBudget:  0, // unlimited
		ToolConcurrency:  4,
		RetryAttempts:    3,
		RetryBaseDelay:   time.Second,
		SessionTTL:       30 * time.Minute,
	}
}
