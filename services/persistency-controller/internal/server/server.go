// Package server implements the PersistencyService gRPC API: cache-aside reads
// and durable writes.
package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	"github.com/amusaev/url-shortener/services/persistency-controller/internal/model"
)

// Repository is the durable store contract.
type Repository interface {
	Create(ctx context.Context, m model.Mapping) (*model.Mapping, bool, error)
	Get(ctx context.Context, code string) (*model.Mapping, error)
}

// CacheLayer is the cache-aside contract.
type CacheLayer interface {
	Get(ctx context.Context, code string) (*model.Mapping, bool, error)
	Set(ctx context.Context, m model.Mapping) error
}

// Server resolves and persists mappings.
type Server struct {
	persistencyv1.UnimplementedPersistencyServiceServer
	repo   Repository
	cache  CacheLayer
	logger *slog.Logger
}

// New constructs the persistency Server.
func New(repo Repository, cache CacheLayer, logger *slog.Logger) *Server {
	return &Server{repo: repo, cache: cache, logger: logger}
}

// CreateMapping durably writes a mapping; a code collision yields AlreadyExists.
func (s *Server) CreateMapping(ctx context.Context, req *persistencyv1.CreateMappingRequest) (*persistencyv1.CreateMappingResponse, error) {
	if req.GetCode() == "" || req.GetLongUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "code and long_url are required")
	}

	m := model.Mapping{
		Code:      req.GetCode(),
		LongURL:   req.GetLongUrl(),
		ExpiresAt: fromProto(req.GetExpiresAt()),
	}

	created, ok, err := s.repo.Create(ctx, m)
	if err != nil {
		s.logger.ErrorContext(ctx, "create failed", slog.String("error", err.Error()))
		return nil, status.Error(codes.Unavailable, "storage unavailable")
	}
	if !ok {
		return nil, status.Errorf(codes.AlreadyExists, "code %q already exists", req.GetCode())
	}

	if err := s.cache.Set(ctx, *created); err != nil {
		s.logger.WarnContext(ctx, "cache warm failed", slog.String("error", err.Error()))
	}
	return &persistencyv1.CreateMappingResponse{Mapping: toProto(created)}, nil
}

// GetMapping reads cache-first and falls back to Postgres on a miss or a cache
// failure (availability over consistency).
func (s *Server) GetMapping(ctx context.Context, req *persistencyv1.GetMappingRequest) (*persistencyv1.GetMappingResponse, error) {
	code := req.GetCode()
	if code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	if m, hit, err := s.cache.Get(ctx, code); err != nil {
		s.logger.WarnContext(ctx, "cache read failed; falling back to db", slog.String("error", err.Error()))
	} else if hit {
		return &persistencyv1.GetMappingResponse{Mapping: toProto(m)}, nil
	}

	m, err := s.repo.Get(ctx, code)
	if errors.Is(err, model.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "code %q not found", code)
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "db read failed", slog.String("error", err.Error()))
		return nil, status.Error(codes.Unavailable, "storage unavailable")
	}

	if err := s.cache.Set(ctx, *m); err != nil {
		s.logger.WarnContext(ctx, "cache populate failed", slog.String("error", err.Error()))
	}
	return &persistencyv1.GetMappingResponse{Mapping: toProto(m)}, nil
}

func toProto(m *model.Mapping) *persistencyv1.Mapping {
	return &persistencyv1.Mapping{
		Code:      m.Code,
		LongUrl:   m.LongURL,
		ExpiresAt: toProtoTS(m.ExpiresAt),
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func toProtoTS(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func fromProto(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
