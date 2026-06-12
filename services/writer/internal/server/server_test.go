package server

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
)

type stubGen struct{ n uint64 }

func (s *stubGen) Next(context.Context) (uint64, error) {
	s.n++
	return s.n, nil
}

type stubPersistency struct {
	existing map[string]bool
	calls    []string
}

func (s *stubPersistency) CreateMapping(_ context.Context, in *persistencyv1.CreateMappingRequest, _ ...grpc.CallOption) (*persistencyv1.CreateMappingResponse, error) {
	s.calls = append(s.calls, in.GetCode())
	if s.existing[in.GetCode()] {
		return nil, status.Errorf(codes.AlreadyExists, "exists")
	}
	s.existing[in.GetCode()] = true
	return &persistencyv1.CreateMappingResponse{Mapping: &persistencyv1.Mapping{Code: in.GetCode()}}, nil
}

func newServer(p PersistencyClient) *Server {
	return New(&stubGen{}, p, "http://localhost:8080", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestShortenGenerated(t *testing.T) {
	p := &stubPersistency{existing: map[string]bool{}}
	resp, err := newServer(p).Shorten(context.Background(), &writerv1.ShortenRequest{LongUrl: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCode() != "1" {
		t.Errorf("code = %q, want %q", resp.GetCode(), "1")
	}
	if resp.GetShortUrl() != "http://localhost:8080/1" {
		t.Errorf("short_url = %q", resp.GetShortUrl())
	}
}

func TestShortenGeneratedRetriesOnAliasCollision(t *testing.T) {
	// Codes "1" and "2" are pre-taken by aliases; writer must skip to "3".
	p := &stubPersistency{existing: map[string]bool{"1": true, "2": true}}
	resp, err := newServer(p).Shorten(context.Background(), &writerv1.ShortenRequest{LongUrl: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCode() != "3" {
		t.Errorf("expected to skip taken codes and land on %q, got %q", "3", resp.GetCode())
	}
}

func TestShortenCustomAlias(t *testing.T) {
	p := &stubPersistency{existing: map[string]bool{}}
	alias := "my-alias"
	resp, err := newServer(p).Shorten(context.Background(), &writerv1.ShortenRequest{
		LongUrl:     "https://example.com",
		CustomAlias: &alias,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCode() != alias {
		t.Errorf("code = %q, want %q", resp.GetCode(), alias)
	}
}

func TestShortenAliasConflict(t *testing.T) {
	p := &stubPersistency{existing: map[string]bool{"taken": true}}
	alias := "taken"
	_, err := newServer(p).Shorten(context.Background(), &writerv1.ShortenRequest{
		LongUrl:     "https://example.com",
		CustomAlias: &alias,
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("want AlreadyExists, got %v", err)
	}
}

func TestShortenRejectsBadURL(t *testing.T) {
	p := &stubPersistency{existing: map[string]bool{}}
	_, err := newServer(p).Shorten(context.Background(), &writerv1.ShortenRequest{LongUrl: "not-a-url"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}
