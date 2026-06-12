package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
)

type fakeWriter struct {
	resp *writerv1.ShortenResponse
	err  error
}

func (f fakeWriter) Shorten(_ context.Context, _ *writerv1.ShortenRequest, _ ...grpc.CallOption) (*writerv1.ShortenResponse, error) {
	return f.resp, f.err
}

type fakeReader struct {
	resp *readerv1.ResolveResponse
	err  error
}

func (f fakeReader) Resolve(_ context.Context, _ *readerv1.ResolveRequest, _ ...grpc.CallOption) (*readerv1.ResolveResponse, error) {
	return f.resp, f.err
}

func testRouter(w writerv1.WriterServiceClient, r readerv1.ReaderServiceClient) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandlers(w, r, logger)
	limiter := NewIPRateLimiter(1000, 1000, time.Minute)
	return NewRouter(h, limiter, "*", logger)
}

func TestCreateSuccess(t *testing.T) {
	w := fakeWriter{resp: &writerv1.ShortenResponse{
		Code:     "abc",
		ShortUrl: "http://localhost:8080/abc",
		LongUrl:  "https://example.com",
	}}
	router := testRouter(w, fakeReader{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"long_url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp urlResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ShortURL != "http://localhost:8080/abc" {
		t.Errorf("short_url = %q", resp.ShortURL)
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	router := testRouter(fakeWriter{}, fakeReader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateConflictMapsTo409(t *testing.T) {
	w := fakeWriter{err: status.Error(codes.AlreadyExists, "alias taken")}
	router := testRouter(w, fakeReader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"long_url":"https://example.com","custom_alias":"taken"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestRedirectFound(t *testing.T) {
	r := fakeReader{resp: &readerv1.ResolveResponse{LongUrl: "https://example.com/dest"}}
	router := testRouter(fakeWriter{}, r)
	req := httptest.NewRequest(http.MethodGet, "/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/dest" {
		t.Errorf("Location = %q", loc)
	}
}

func TestRedirectNotFound(t *testing.T) {
	r := fakeReader{err: status.Error(codes.NotFound, "missing")}
	router := testRouter(fakeWriter{}, r)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestRedirectExpiredMapsTo410(t *testing.T) {
	r := fakeReader{err: status.Error(codes.FailedPrecondition, "expired")}
	router := testRouter(fakeWriter{}, r)
	req := httptest.NewRequest(http.MethodGet, "/old", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
}
