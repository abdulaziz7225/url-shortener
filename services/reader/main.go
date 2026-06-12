package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/amusaev/url-shortener/libs/config"
	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
	"github.com/amusaev/url-shortener/libs/grpcutil"
	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/libs/service"
	"github.com/amusaev/url-shortener/services/reader/internal/server"
)

const serviceName = "reader"

// getMappingMethod is idempotent and therefore safe to retry.
const getMappingMethod = "/persistency.v1.PersistencyService/GetMapping"

func main() {
	logger := observability.NewLogger(serviceName)
	ctx := context.Background()

	clientCfg := grpcutil.ClientConfig{
		Timeout:           config.Duration("DOWNSTREAM_TIMEOUT", 2*time.Second),
		RetryAttempts:     config.Int("RETRY_ATTEMPTS", 3),
		RetryBackoff:      config.Duration("RETRY_BACKOFF", 20*time.Millisecond),
		IdempotentMethods: map[string]bool{getMappingMethod: true},
	}

	persistencyConn, err := grpcutil.Dial(config.String("PERSISTENCY_ADDR", "persistency-controller:9090"), serviceName, clientCfg)
	if err != nil {
		logger.ErrorContext(ctx, "persistency dial failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = persistencyConn.Close() }()

	srv := server.New(persistencyv1.NewPersistencyServiceClient(persistencyConn), logger)

	readiness := &observability.Readiness{}
	readiness.SetReady(true)

	cfg := service.Config{
		Name:         serviceName,
		GRPCAddr:     config.String("GRPC_ADDR", ":9090"),
		OpsAddr:      config.String("OPS_ADDR", ":9091"),
		OTLPEndpoint: config.String("OTLP_ENDPOINT", ""),
		Logger:       logger,
		Readiness:    readiness,
		Register: func(gs *grpc.Server) {
			readerv1.RegisterReaderServiceServer(gs, srv)
		},
	}

	if err := service.Run(ctx, cfg); err != nil {
		logger.ErrorContext(ctx, "service stopped with error", "error", err)
		os.Exit(1)
	}
}
