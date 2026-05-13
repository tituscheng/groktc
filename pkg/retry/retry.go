// Package retry provides generic retry logic with exponential backoff.
package retry

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Policy configures retry behavior.
type Policy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

// DefaultPolicy is a sensible default: 3 attempts, 250ms base delay.
var DefaultPolicy = Policy{
	MaxAttempts: 3,
	BaseDelay:   250 * time.Millisecond,
}

// IsRetryable decides whether an error warrants another attempt.
type IsRetryable func(error) bool

// Operation is the work to retry.
type Operation func() error

// Do runs operation with retries according to policy.
// It logs warnings for each failed attempt and an error after exhaustion.
func Do(ctx context.Context, policy Policy, isRetryable IsRetryable, operation Operation) error {
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == policy.MaxAttempts {
			break
		}
		slog.Warn("retryable operation failed, retrying",
			"attempt", attempt,
			"max_attempts", policy.MaxAttempts,
			"error", err,
		)
		if sleepErr := Sleep(ctx, Backoff(policy.BaseDelay, attempt)); sleepErr != nil {
			return sleepErr
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", policy.MaxAttempts, lastErr)
}

// Backoff returns the delay for a given attempt using exponential backoff.
func Backoff(base time.Duration, attempt int) time.Duration {
	return time.Duration(base) * time.Duration(1<<(attempt-1))
}

// Sleep blocks until the duration elapses or the context is canceled.
func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
