package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/amusaev/url-shortener/libs/config"
	counterv1 "github.com/amusaev/url-shortener/libs/gen/counter/v1"
	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
	"github.com/amusaev/url-shortener/libs/grpcutil"
	"github.com/amusaev/url-shortener/libs/observability"
	"github.com/amusaev/url-shortener/libs/service"
	"github.com/amusaev/url-shortener/services/writer/internal/codegen"
	"github.com/amusaev/url-shortener/services/writer/internal/server"
)

const serviceName = "writer"

type counterAllocator struct {
	client counterv1.CounterServiceClient
}

func (a counterAllocator) Allocate(ctx context.Context, batchSize uint64) (uint64, uint64, error) {
	resp, err := a.client.AllocateRange(ctx, &counterv1.AllocateRangeRequest{BatchSize: batchSize})
	if err != nil {
		return 0, 0, err
	}
	return resp.GetStart(), resp.GetEnd(), nil
}

func main() {
	logger := observability.NewLogger(serviceName)
	ctx := context.Background()

	clientCfg := grpcutil.ClientConfig{Timeout: config.Duration("DOWNSTREAM_TIMEOUT", 2*time.Second)}

	counterConn, err := grpcutil.Dial(config.String("COUNTER_ADDR", "counter:9090"), serviceName, clientCfg)
	if err != nil {
		logger.ErrorContext(ctx, "counter dial failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = counterConn.Close() }()

	persistencyConn, err := grpcutil.Dial(config.String("PERSISTENCY_ADDR", "persistency-controller:9090"), serviceName, clientCfg)
	if err != nil {
		logger.ErrorContext(ctx, "persistency dial failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = persistencyConn.Close() }()

	gen := codegen.New(
		counterAllocator{client: counterv1.NewCounterServiceClient(counterConn)},
		uint64(config.Int("BATCH_SIZE", 1000)),
	)
	srv := server.New(
		gen,
		persistencyv1.NewPersistencyServiceClient(persistencyConn),
		config.String("SHORT_URL_BASE", "http://localhost:8080"),
		logger,
	)

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
			writerv1.RegisterWriterServiceServer(gs, srv)
		},
	}

	if err := service.Run(ctx, cfg); err != nil {
		logger.ErrorContext(ctx, "service stopped with error", "error", err)
		os.Exit(1)
	}
}
