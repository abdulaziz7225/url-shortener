package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	counterv1 "github.com/amusaev/url-shortener/libs/gen/counter/v1"
)

type fakeAllocator struct {
	next uint64
	err  error
}

func (f *fakeAllocator) Allocate(_ context.Context, batchSize uint64) (uint64, uint64, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	start := f.next + 1
	f.next += batchSize
	return start, f.next, nil
}

func testServer(a *fakeAllocator) *Server {
	return New(a, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAllocateRangeDisjoint(t *testing.T) {
	s := testServer(&fakeAllocator{})
	r1, err := s.AllocateRange(context.Background(), &counterv1.AllocateRangeRequest{BatchSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := s.AllocateRange(context.Background(), &counterv1.AllocateRangeRequest{BatchSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if r1.GetStart() != 1 || r1.GetEnd() != 1000 {
		t.Errorf("first range = [%d,%d], want [1,1000]", r1.GetStart(), r1.GetEnd())
	}
	if r2.GetStart() != 1001 || r2.GetEnd() != 2000 {
		t.Errorf("second range = [%d,%d], want [1001,2000]", r2.GetStart(), r2.GetEnd())
	}
}

func TestAllocateRangeRejectsZero(t *testing.T) {
	s := testServer(&fakeAllocator{})
	_, err := s.AllocateRange(context.Background(), &counterv1.AllocateRangeRequest{BatchSize: 0})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestAllocateRangeStoreError(t *testing.T) {
	s := testServer(&fakeAllocator{err: errors.New("redis down")})
	_, err := s.AllocateRange(context.Background(), &counterv1.AllocateRangeRequest{BatchSize: 10})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
}
