package grpcutil

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sony/gobreaker"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/libs/resilience"
)

// ClientConfig tunes the resilience behaviour of an outbound gRPC connection.
type ClientConfig struct {
	// Timeout bounds each logical call (including its retries).
	Timeout time.Duration
	// RetryAttempts is the maximum attempts for idempotent methods.
	RetryAttempts int
	// RetryBackoff is the linear backoff base between retries.
	RetryBackoff time.Duration
	// IdempotentMethods lists full method names ("/pkg.Service/Method") that are
	// safe to retry. Only reads belong here.
	IdempotentMethods map[string]bool
}

// Dial opens a client connection wrapped with tracing, metrics, per-call
// timeouts, idempotent-read retries, and a per-target circuit breaker.
func Dial(target, service string, cfg ClientConfig) (*grpc.ClientConn, error) {
	if !strings.Contains(target, "://") {
		target = "dns:///" + target
	}
	breaker := resilience.NewBreaker(service + " -> " + target)

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			observability.MetricsUnaryClientInterceptor(service),
			timeoutUnaryClientInterceptor(cfg.Timeout),
			breakerUnaryClientInterceptor(breaker),
			retryUnaryClientInterceptor(cfg),
		),
	)
	if err != nil {
		return nil, err
	}
	// Connect eagerly so the first real request does not pay connection setup
	// inside its deadline.
	conn.Connect()
	return conn, nil
}

func timeoutUnaryClientInterceptor(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if d <= 0 {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func breakerUnaryClientInterceptor(cb *gobreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		_, err := cb.Execute(func() (any, error) {
			return nil, invoker(ctx, method, req, reply, cc, opts...)
		})
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return status.Error(codes.Unavailable, "circuit breaker open for "+method)
		}
		return err
	}
}

func retryUnaryClientInterceptor(cfg ClientConfig) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		attempts := 1
		if cfg.IdempotentMethods[method] && cfg.RetryAttempts > 1 {
			attempts = cfg.RetryAttempts
		}
		backoff := cfg.RetryBackoff
		if backoff <= 0 {
			backoff = 20 * time.Millisecond
		}
		return resilience.Retry(ctx, attempts, backoff, func(ctx context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}
