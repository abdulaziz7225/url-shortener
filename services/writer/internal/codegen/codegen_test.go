package codegen

import (
	"context"
	"errors"
	"testing"
)

type stubAllocator struct {
	cursor uint64
	calls  int
	err    error
}

func (s *stubAllocator) Allocate(_ context.Context, batchSize uint64) (uint64, uint64, error) {
	if s.err != nil {
		return 0, 0, s.err
	}
	s.calls++
	start := s.cursor + 1
	s.cursor += batchSize
	return start, s.cursor, nil
}

func TestNextRefillsAcrossBatchBoundary(t *testing.T) {
	alloc := &stubAllocator{}
	g := New(alloc, 3)

	var got []uint64
	for i := 0; i < 7; i++ {
		v, err := g.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}

	want := []uint64{1, 2, 3, 4, 5, 6, 7}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("value %d = %d, want %d", i, got[i], want[i])
		}
	}
	if alloc.calls != 3 {
		t.Errorf("expected 3 batch allocations for 7 values at batch=3, got %d", alloc.calls)
	}
}

func TestNextPropagatesAllocatorError(t *testing.T) {
	g := New(&stubAllocator{err: errors.New("counter down")}, 10)
	if _, err := g.Next(context.Background()); err == nil {
		t.Fatal("expected error when allocator fails")
	}
}

func TestValuesAreUnique(t *testing.T) {
	g := New(&stubAllocator{}, 50)
	seen := make(map[uint64]bool)
	for i := 0; i < 500; i++ {
		v, err := g.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if seen[v] {
			t.Fatalf("duplicate value %d", v)
		}
		seen[v] = true
	}
}
