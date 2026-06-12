// Package service provides the common bootstrap shared by every gRPC service:
// tracing init, server wiring, the ops/metrics endpoint, and graceful shutdown.
package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/amusaev/url-shortener/libs/grpcutil"
	"github.com/amusaev/url-shortener/libs/observability"
)

// Config describes how to run a gRPC service.
type Config struct {
	Name         string
	GRPCAddr     string
	OpsAddr      string
	OTLPEndpoint string
	Logger       *slog.Logger
	Readiness    *observability.Readiness
	// Register attaches service implementations to the gRPC server.
	Register func(*grpc.Server)
}

// Run starts the gRPC server and ops endpoint, blocks until a termination
// signal or fatal error, then shuts everything down gracefully.
func Run(ctx context.Context, cfg Config) error {
	shutdownTracing, err := observability.InitTracing(ctx, cfg.Name, cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()

	grpcServer := grpcutil.NewServer(cfg.Name, cfg.Logger)
	cfg.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	ops := observability.NewOpsServer(cfg.OpsAddr, cfg.Readiness)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 2)
	go func() { errc <- grpcServer.Serve(lis) }()
	go func() {
		if err := ops.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	cfg.Logger.InfoContext(ctx, "service started",
		slog.String("grpc_addr", cfg.GRPCAddr),
		slog.String("ops_addr", cfg.OpsAddr),
	)

	select {
	case <-ctx.Done():
		cfg.Logger.InfoContext(context.Background(), "shutdown signal received")
	case err := <-errc:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	return ops.Shutdown(shutCtx)
}
