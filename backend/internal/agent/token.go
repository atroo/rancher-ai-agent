package agent

import (
	"errors"
	"sync"
)

// ErrBudgetExceeded is returned when the token budget is exhausted.
var ErrBudgetExceeded = errors.New("token budget exceeded")

// TokenBudget tracks cumulative token usage against an optional limit.
type TokenBudget struct {
	limit int // 0 = unlimited
	used  int
	mu    sync.Mutex
}

// NewTokenBudget creates a budget. Pass 0 for unlimited.
func NewTokenBudget(limit int) *TokenBudget {
	return &TokenBudget{limit: limit}
}

// Record adds tokens to the running total.
// Returns ErrBudgetExceeded if the limit is exceeded (tokens are still recorded).
func (tb *TokenBudget) Record(tokens int) error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.used += tokens
	if tb.limit > 0 && tb.used > tb.limit {
		return ErrBudgetExceeded
	}
	return nil
}

// Used returns the total tokens consumed so far.
func (tb *TokenBudget) Used() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.used
}

// Remaining returns how many tokens are left, or -1 if unlimited.
func (tb *TokenBudget) Remaining() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.limit <= 0 {
		return -1
	}
	r := tb.limit - tb.used
	if r < 0 {
		return 0
	}
	return r
}
