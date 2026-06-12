// Package server implements the CounterService gRPC API.
package server

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	counterv1 "github.com/amusaev/url-shortener/libs/gen/counter/v1"
	"github.com/amusaev/url-shortener/services/counter/internal/store"
)

const maxBatchSize = 100_000

// Server serves counter range allocations.
type Server struct {
	counterv1.UnimplementedCounterServiceServer
	alloc  store.Allocator
	logger *slog.Logger
}

// New constructs a counter Server.
func New(alloc store.Allocator, logger *slog.Logger) *Server {
	return &Server{alloc: alloc, logger: logger}
}

// AllocateRange reserves a disjoint counter range for a writer.
func (s *Server) AllocateRange(ctx context.Context, req *counterv1.AllocateRangeRequest) (*counterv1.AllocateRangeResponse, error) {
	batch := req.GetBatchSize()
	if batch == 0 {
		return nil, status.Error(codes.InvalidArgument, "batch_size must be greater than zero")
	}
	if batch > maxBatchSize {
		return nil, status.Errorf(codes.InvalidArgument, "batch_size exceeds maximum of %d", maxBatchSize)
	}

	start, end, err := s.alloc.Allocate(ctx, batch)
	if err != nil {
		s.logger.ErrorContext(ctx, "allocate failed", slog.String("error", err.Error()))
		return nil, status.Error(codes.Unavailable, "counter store unavailable")
	}
	return &counterv1.AllocateRangeResponse{Start: start, End: end}, nil
}
