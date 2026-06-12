package main

import (
	"context"
	"os"

	"google.golang.org/grpc"

	"github.com/amusaev/url-shortener/libs/config"
	counterv1 "github.com/amusaev/url-shortener/libs/gen/counter/v1"
	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/libs/redisx"
	"github.com/amusaev/url-shortener/libs/service"
	"github.com/amusaev/url-shortener/services/counter/internal/server"
	"github.com/amusaev/url-shortener/services/counter/internal/store"
)

const serviceName = "counter"

func main() {
	logger := observability.NewLogger(serviceName)
	ctx := context.Background()

	rdb, err := redisx.New(ctx, config.String("REDIS_ADDR", "redis:6379"))
	if err != nil {
		logger.ErrorContext(ctx, "redis connect failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	alloc := store.NewRedisAllocator(rdb, config.String("COUNTER_KEY", "counter:sequence"))
	srv := server.New(alloc, logger)

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
			counterv1.RegisterCounterServiceServer(gs, srv)
		},
	}

	if err := service.Run(ctx, cfg); err != nil {
		logger.ErrorContext(ctx, "service stopped with error", "error", err)
		os.Exit(1)
	}
}
