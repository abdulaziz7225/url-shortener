package server

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
)

type stubPersistency struct {
	mapping *persistencyv1.Mapping
	err     error
}

func (s stubPersistency) GetMapping(_ context.Context, _ *persistencyv1.GetMappingRequest, _ ...grpc.CallOption) (*persistencyv1.GetMappingResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &persistencyv1.GetMappingResponse{Mapping: s.mapping}, nil
}

func newServer(p PersistencyClient, now time.Time) *Server {
	s := New(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	return s
}

func TestResolveActive(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	p := stubPersistency{mapping: &persistencyv1.Mapping{
		Code:      "abc",
		LongUrl:   "https://example.com",
		ExpiresAt: timestamppb.New(now.Add(time.Hour)),
	}}
	resp, err := newServer(p, now).Resolve(context.Background(), &readerv1.ResolveRequest{Code: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetLongUrl() != "https://example.com" {
		t.Errorf("long_url = %q", resp.GetLongUrl())
	}
}

func TestResolveNoExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	p := stubPersistency{mapping: &persistencyv1.Mapping{Code: "abc", LongUrl: "https://example.com"}}
	if _, err := newServer(p, now).Resolve(context.Background(), &readerv1.ResolveRequest{Code: "abc"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveExpired(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	p := stubPersistency{mapping: &persistencyv1.Mapping{
		Code:      "abc",
		LongUrl:   "https://example.com",
		ExpiresAt: timestamppb.New(now.Add(-time.Second)),
	}}
	_, err := newServer(p, now).Resolve(context.Background(), &readerv1.ResolveRequest{Code: "abc"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition for expired, got %v", err)
	}
}

func TestResolvePropagatesNotFound(t *testing.T) {
	p := stubPersistency{err: status.Error(codes.NotFound, "missing")}
	_, err := newServer(p, time.Now()).Resolve(context.Background(), &readerv1.ResolveRequest{Code: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestResolveRejectsEmptyCode(t *testing.T) {
	p := stubPersistency{}
	_, err := newServer(p, time.Now()).Resolve(context.Background(), &readerv1.ResolveRequest{Code: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}
