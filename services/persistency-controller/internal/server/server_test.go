package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	persistencyv1 "github.com/amusaev/url-shortener/libs/gen/persistency/v1"
	"github.com/amusaev/url-shortener/services/persistency-controller/internal/model"
)

type fakeRepo struct {
	store     map[string]model.Mapping
	getCalls  int
	createErr error
	getErr    error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{store: map[string]model.Mapping{}} }

func (f *fakeRepo) Create(_ context.Context, m model.Mapping) (*model.Mapping, bool, error) {
	if f.createErr != nil {
		return nil, false, f.createErr
	}
	if _, ok := f.store[m.Code]; ok {
		return nil, false, nil
	}
	f.store[m.Code] = m
	return &m, true, nil
}

func (f *fakeRepo) Get(_ context.Context, code string) (*model.Mapping, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	m, ok := f.store[code]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &m, nil
}

type fakeCache struct {
	store    map[string]model.Mapping
	getErr   error
	setCalls int
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string]model.Mapping{}} }

func (f *fakeCache) Get(_ context.Context, code string) (*model.Mapping, bool, error) {
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	m, ok := f.store[code]
	if !ok {
		return nil, false, nil
	}
	return &m, true, nil
}

func (f *fakeCache) Set(_ context.Context, m model.Mapping) error {
	f.setCalls++
	f.store[m.Code] = m
	return nil
}

func testServer(repo Repository, cache CacheLayer) *Server {
	return New(repo, cache, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateMappingThenConflict(t *testing.T) {
	repo, cache := newFakeRepo(), newFakeCache()
	s := testServer(repo, cache)
	req := &persistencyv1.CreateMappingRequest{Code: "abc", LongUrl: "https://example.com"}

	if _, err := s.CreateMapping(context.Background(), req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if cache.setCalls != 1 {
		t.Errorf("expected cache warm on create, got %d sets", cache.setCalls)
	}
	_, err := s.CreateMapping(context.Background(), req)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("want AlreadyExists, got %v", err)
	}
}

func TestGetMappingCacheHitSkipsDB(t *testing.T) {
	repo, cache := newFakeRepo(), newFakeCache()
	cache.store["abc"] = model.Mapping{Code: "abc", LongURL: "https://cached.example"}
	s := testServer(repo, cache)

	resp, err := s.GetMapping(context.Background(), &persistencyv1.GetMappingRequest{Code: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetMapping().GetLongUrl() != "https://cached.example" {
		t.Errorf("unexpected url %q", resp.GetMapping().GetLongUrl())
	}
	if repo.getCalls != 0 {
		t.Errorf("cache hit must not touch db, got %d db calls", repo.getCalls)
	}
}

func TestGetMappingCacheMissPopulatesFromDB(t *testing.T) {
	repo, cache := newFakeRepo(), newFakeCache()
	repo.store["abc"] = model.Mapping{Code: "abc", LongURL: "https://db.example"}
	s := testServer(repo, cache)

	resp, err := s.GetMapping(context.Background(), &persistencyv1.GetMappingRequest{Code: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetMapping().GetLongUrl() != "https://db.example" {
		t.Errorf("unexpected url %q", resp.GetMapping().GetLongUrl())
	}
	if cache.setCalls != 1 {
		t.Errorf("expected cache populate after db read, got %d", cache.setCalls)
	}
}

func TestGetMappingFallsBackWhenCacheFails(t *testing.T) {
	repo, cache := newFakeRepo(), newFakeCache()
	repo.store["abc"] = model.Mapping{Code: "abc", LongURL: "https://db.example"}
	cache.getErr = errors.New("redis down")
	s := testServer(repo, cache)

	resp, err := s.GetMapping(context.Background(), &persistencyv1.GetMappingRequest{Code: "abc"})
	if err != nil {
		t.Fatalf("cache failure should fall back to db, got %v", err)
	}
	if resp.GetMapping().GetLongUrl() != "https://db.example" {
		t.Errorf("unexpected url %q", resp.GetMapping().GetLongUrl())
	}
}

func TestGetMappingNotFound(t *testing.T) {
	s := testServer(newFakeRepo(), newFakeCache())
	_, err := s.GetMapping(context.Background(), &persistencyv1.GetMappingRequest{Code: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}
