// Package resilience holds the infra-failure classification, circuit breaker,
// and retry helpers shared across services.
package resilience

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsInfraError reports whether err represents an infrastructure failure (a dead
// or overloaded downstream) as opposed to a business outcome. Circuit breakers
// trip and retries fire only on infra errors; business errors pass straight
// through untouched.
func IsInfraError(err error) bool {
	if err == nil {
		return false
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted,
		codes.Internal, codes.Unknown, codes.DataLoss:
		return true
	default:
		return false
	}
}

// NewBreaker builds a circuit breaker that ignores business errors and trips
// only after a run of infrastructure failures.
func NewBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			// Only infra errors count against the breaker.
			return !IsInfraError(err)
		},
	})
}

// Retry invokes fn up to attempts times, retrying only when the returned error
// is an infrastructure failure. It is intended for idempotent reads. Backoff
// grows linearly and stops early if the context is cancelled.
func Retry(ctx context.Context, attempts int, backoff time.Duration, fn func(context.Context) error) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(ctx); err == nil || !IsInfraError(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * backoff):
		}
	}
	return err
}
