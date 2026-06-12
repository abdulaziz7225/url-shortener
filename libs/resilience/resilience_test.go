package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsInfraError(t *testing.T) {
	infra := []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal}
	for _, c := range infra {
		if !IsInfraError(status.Error(c, "x")) {
			t.Errorf("%v should be infra", c)
		}
	}
	business := []codes.Code{codes.NotFound, codes.AlreadyExists, codes.InvalidArgument, codes.FailedPrecondition}
	for _, c := range business {
		if IsInfraError(status.Error(c, "x")) {
			t.Errorf("%v should not be infra", c)
		}
	}
	if IsInfraError(nil) {
		t.Error("nil is not infra")
	}
}

func TestRetryRetriesInfraThenSucceeds(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 3, time.Millisecond, func(context.Context) error {
		calls++
		if calls < 3 {
			return status.Error(codes.Unavailable, "down")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryDoesNotRetryBusinessError(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 3, time.Millisecond, func(context.Context) error {
		calls++
		return status.Error(codes.NotFound, "missing")
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("business error must not retry, got %d calls", calls)
	}
}

func TestRetryStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Retry(ctx, 5, time.Second, func(context.Context) error {
		return status.Error(codes.Unavailable, "down")
	})
	if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Unavailable {
		t.Fatalf("unexpected error: %v", err)
	}
}
