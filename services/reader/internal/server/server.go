// Package server implements the ReaderService gRPC API on the hot path.
package server

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
)

// PersistencyClient is the subset of the persistency API the reader needs.
type PersistencyClient interface {
	GetMapping(ctx context.Context, in *persistencyv1.GetMappingRequest, opts ...grpc.CallOption) (*persistencyv1.GetMappingResponse, error)
}

// Server resolves short codes to long URLs, enforcing expiry at read time.
type Server struct {
	readerv1.UnimplementedReaderServiceServer
	persistency PersistencyClient
	now         func() time.Time
	logger      *slog.Logger
}

// New constructs a reader Server.
func New(persistency PersistencyClient, logger *slog.Logger) *Server {
	return &Server{persistency: persistency, now: time.Now, logger: logger}
}

// Resolve returns the long URL for a code. Unknown codes yield NotFound and
// expired codes yield FailedPrecondition (mapped to HTTP 410 at the edge).
func (s *Server) Resolve(ctx context.Context, req *readerv1.ResolveRequest) (*readerv1.ResolveResponse, error) {
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	resp, err := s.persistency.GetMapping(ctx, &persistencyv1.GetMappingRequest{Code: req.GetCode()})
	if err != nil {
		// NotFound and infrastructure errors propagate unchanged.
		return nil, err
	}

	m := resp.GetMapping()
	if expiry := m.GetExpiresAt(); expiry != nil && !expiry.AsTime().After(s.now()) {
		return nil, status.Errorf(codes.FailedPrecondition, "code %q has expired", req.GetCode())
	}

	return &readerv1.ResolveResponse{
		LongUrl:   m.GetLongUrl(),
		ExpiresAt: m.GetExpiresAt(),
	}, nil
}
