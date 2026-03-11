package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// retryProvider wraps a Provider with exponential backoff retry logic.
type retryProvider struct {
	inner    Provider
	attempts int
	base     time.Duration
}

// WithRetry wraps a provider with retry logic for transient errors.
func WithRetry(p Provider, attempts int, base time.Duration) Provider {
	if attempts <= 1 {
		return p
	}
	return &retryProvider{inner: p, attempts: attempts, base: base}
}

func (r *retryProvider) CreateMessage(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	var lastErr error

	for attempt := range r.attempts {
		resp, err := r.inner.CreateMessage(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}

		delay := r.base * time.Duration(1<<uint(attempt))
		slog.Warn("provider call failed, retrying",
			"attempt", attempt+1,
			"max_attempts", r.attempts,
			"delay", delay,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}

func (r *retryProvider) StreamMessage(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, *ProviderResponse, error) {
	var lastErr error

	for attempt := range r.attempts {
		events, resp, err := r.inner.StreamMessage(ctx, req)
		if err == nil {
			return events, resp, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, nil, err
		}

		delay := r.base * time.Duration(1<<uint(attempt))
		slog.Warn("provider stream failed, retrying",
			"attempt", attempt+1,
			"max_attempts", r.attempts,
			"delay", delay,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, nil, lastErr
}

// isRetryable checks if an error is transient and worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	// Rate limited
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate") {
		return true
	}

	// Server errors
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	// Network errors
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "connection") {
		return true
	}

	return false
}
