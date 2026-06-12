package grpcutil

import (
	"context"
	"log/slog"
	"runtime/debug"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/amusaev/url-shortener/libs/observability"
)

// NewServer builds a gRPC server pre-wired with OpenTelemetry tracing, Prometheus
// metrics, panic recovery, and request logging.
func NewServer(service string, logger *slog.Logger) *grpc.Server {
	return grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recoveryUnaryServerInterceptor(logger),
			observability.MetricsUnaryServerInterceptor(service),
			loggingUnaryServerInterceptor(logger),
		),
	)
}

func recoveryUnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic recovered",
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func loggingUnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		code := status.Code(err)
		if err != nil {
			logger.WarnContext(ctx, "grpc call failed",
				slog.String("method", info.FullMethod),
				slog.String("code", code.String()),
				slog.String("error", err.Error()),
			)
		} else {
			logger.InfoContext(ctx, "grpc call",
				slog.String("method", info.FullMethod),
				slog.String("code", code.String()),
			)
		}
		return resp, err
	}
}
