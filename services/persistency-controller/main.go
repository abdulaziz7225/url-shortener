package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/amusaev/url-shortener/libs/config"
	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/libs/redisx"
	"github.com/amusaev/url-shortener/libs/service"
	"github.com/amusaev/url-shortener/services/persistency-controller/internal/cache"
	"github.com/amusaev/url-shortener/services/persistency-controller/internal/server"
	"github.com/amusaev/url-shortener/services/persistency-controller/internal/store"
)

const serviceName = "persistency-controller"

func main() {
	logger := observability.NewLogger(serviceName)
	ctx := context.Background()

	dsn := config.String("POSTGRES_DSN",
		"postgres://urlshortener:urlshortener@postgres:5432/urlshortener?sslmode=disable")
	pg, err := store.NewPostgres(ctx, dsn)
	if err != nil {
		logger.ErrorContext(ctx, "postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	if err := pg.Migrate(ctx); err != nil {
		logger.ErrorContext(ctx, "migrations failed", "error", err)
		os.Exit(1)
	}

	rdb, err := redisx.New(ctx, config.String("REDIS_ADDR", "redis:6379"))
	if err != nil {
		logger.ErrorContext(ctx, "redis connect failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	c := cache.New(rdb, config.Duration("CACHE_TTL", time.Hour))
	srv := server.New(pg, c, logger)

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
			persistencyv1.RegisterPersistencyServiceServer(gs, srv)
		},
	}

	if err := service.Run(ctx, cfg); err != nil {
		logger.ErrorContext(ctx, "service stopped with error", "error", err)
		os.Exit(1)
	}
}
