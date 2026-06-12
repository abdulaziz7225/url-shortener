// Package server implements the WriterService gRPC API.
package server

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/amusaev/url-shortener/libs/base62"
	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
	"github.com/amusaev/url-shortener/services/writer/internal/validation"
)

// maxCodeAttempts bounds retries when a generated code collides with an
// existing custom alias (rare; resolved by taking the next counter value).
const maxCodeAttempts = 5

// CodeGenerator yields unique counter values for short codes.
type CodeGenerator interface {
	Next(ctx context.Context) (uint64, error)
}

// PersistencyClient is the subset of the persistency API the writer needs.
type PersistencyClient interface {
	CreateMapping(ctx context.Context, in *persistencyv1.CreateMappingRequest, opts ...grpc.CallOption) (*persistencyv1.CreateMappingResponse, error)
}

// Server creates short-code mappings.
type Server struct {
	writerv1.UnimplementedWriterServiceServer
	gen         CodeGenerator
	persistency PersistencyClient
	shortBase   string
	logger      *slog.Logger
}

// New constructs a writer Server. shortBase is the public origin used to build
// returned short URLs, e.g. "http://localhost:8080".
func New(gen CodeGenerator, persistency PersistencyClient, shortBase string, logger *slog.Logger) *Server {
	return &Server{gen: gen, persistency: persistency, shortBase: shortBase, logger: logger}
}

// Shorten validates input and creates a mapping, either under a custom alias or
// a generated base62 code.
func (s *Server) Shorten(ctx context.Context, req *writerv1.ShortenRequest) (*writerv1.ShortenResponse, error) {
	if err := validation.LongURL(req.GetLongUrl()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var expiresAt *timestamppb.Timestamp
	if req.GetExpiresAt() != nil {
		if err := validation.Expiry(req.GetExpiresAt().AsTime(), time.Now()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		expiresAt = req.GetExpiresAt()
	}

	if alias := req.GetCustomAlias(); alias != "" {
		return s.shortenWithAlias(ctx, alias, req.GetLongUrl(), expiresAt)
	}
	return s.shortenGenerated(ctx, req.GetLongUrl(), expiresAt)
}

func (s *Server) shortenWithAlias(ctx context.Context, alias, longURL string, expiresAt *timestamppb.Timestamp) (*writerv1.ShortenResponse, error) {
	if err := validation.Alias(alias); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	_, err := s.persistency.CreateMapping(ctx, &persistencyv1.CreateMappingRequest{
		Code:      alias,
		LongUrl:   longURL,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		// AlreadyExists propagates to the client as a 409 conflict.
		return nil, err
	}
	return s.response(alias, longURL, expiresAt), nil
}

func (s *Server) shortenGenerated(ctx context.Context, longURL string, expiresAt *timestamppb.Timestamp) (*writerv1.ShortenResponse, error) {
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		value, err := s.gen.Next(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "code generation failed", slog.String("error", err.Error()))
			return nil, status.Error(codes.Unavailable, "could not allocate short code")
		}
		code := base62.Encode(value)

		_, err = s.persistency.CreateMapping(ctx, &persistencyv1.CreateMappingRequest{
			Code:      code,
			LongUrl:   longURL,
			ExpiresAt: expiresAt,
		})
		if err == nil {
			return s.response(code, longURL, expiresAt), nil
		}
		if status.Code(err) == codes.AlreadyExists {
			// Generated code collided with an existing custom alias; advance.
			s.logger.WarnContext(ctx, "generated code collision, retrying", slog.String("code", code))
			continue
		}
		return nil, err
	}
	return nil, status.Error(codes.Internal, "could not allocate a unique short code")
}

func (s *Server) response(code, longURL string, expiresAt *timestamppb.Timestamp) *writerv1.ShortenResponse {
	return &writerv1.ShortenResponse{
		Code:      code,
		ShortUrl:  s.shortBase + "/" + code,
		LongUrl:   longURL,
		ExpiresAt: expiresAt,
	}
}
