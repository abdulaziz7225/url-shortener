package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amusaev/url-shortener/libs/config"
	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
	"github.com/amusaev/url-shortener/libs/grpcutil"
	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/services/api-gateway/internal/httpapi"
)

const (
	serviceName   = "api-gateway"
	resolveMethod = "/reader.v1.ReaderService/Resolve"
)

func main() {
	logger := observability.NewLogger(serviceName)
	ctx := context.Background()

	shutdownTracing, err := observability.InitTracing(ctx, serviceName, config.String("OTLP_ENDPOINT", ""))
	if err != nil {
		logger.ErrorContext(ctx, "tracing init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(c)
	}()

	timeout := config.Duration("DOWNSTREAM_TIMEOUT", 2*time.Second)

	writerConn, err := grpcutil.Dial(config.String("WRITER_ADDR", "writer:9090"), serviceName,
		grpcutil.ClientConfig{Timeout: timeout})
	if err != nil {
		logger.ErrorContext(ctx, "writer dial failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = writerConn.Close() }()

	readerConn, err := grpcutil.Dial(config.String("READER_ADDR", "reader:9090"), serviceName,
		grpcutil.ClientConfig{
			Timeout:           timeout,
			RetryAttempts:     config.Int("RETRY_ATTEMPTS", 3),
			RetryBackoff:      config.Duration("RETRY_BACKOFF", 20*time.Millisecond),
			IdempotentMethods: map[string]bool{resolveMethod: true},
		})
	if err != nil {
		logger.ErrorContext(ctx, "reader dial failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = readerConn.Close() }()

	handlers := httpapi.NewHandlers(
		writerv1.NewWriterServiceClient(writerConn),
		readerv1.NewReaderServiceClient(readerConn),
		logger,
	)
	limiter := httpapi.NewIPRateLimiter(
		float64(config.Int("RATE_LIMIT_RPS", 50)),
		config.Int("RATE_LIMIT_BURST", 100),
		10*time.Minute,
	)
	stopEviction := make(chan struct{})
	go limiter.RunEviction(stopEviction)
	defer close(stopEviction)

	router := httpapi.NewRouter(handlers, limiter, config.String("CORS_ORIGIN", "*"), logger)

	readiness := &observability.Readiness{}
	readiness.SetReady(true)
	ops := observability.NewOpsServer(config.String("OPS_ADDR", ":8081"), readiness)

	httpServer := &http.Server{
		Addr:              config.String("HTTP_ADDR", ":8080"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 2)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	go func() {
		if err := ops.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	logger.InfoContext(ctx, "service started",
		"http_addr", httpServer.Addr,
		"ops_addr", config.String("OPS_ADDR", ":8081"),
	)

	select {
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutdown signal received")
	case err := <-errc:
		logger.ErrorContext(ctx, "server error", "error", err)
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutCtx)
	_ = ops.Shutdown(shutCtx)
}
