// Package codegen turns counter values into short codes, fetching counter
// ranges in batches so most requests need no network round trip.
package codegen

import (
	"context"
	"sync"
)

// Allocator obtains a disjoint counter range of the requested size.
type Allocator interface {
	Allocate(ctx context.Context, batchSize uint64) (start, end uint64, err error)
}

// Generator hands out monotonically increasing counter values, refilling its
// in-memory window [next, max] from the Allocator when exhausted. It is safe
// for concurrent use.
type Generator struct {
	mu        sync.Mutex
	next      uint64
	max       uint64
	exhausted bool
	batchSize uint64
	alloc     Allocator
}

// New builds a Generator that refills batchSize values at a time.
func New(alloc Allocator, batchSize uint64) *Generator {
	if batchSize == 0 {
		batchSize = 1000
	}
	return &Generator{alloc: alloc, batchSize: batchSize, exhausted: true}
}

// Next returns the next counter value, allocating a fresh range if the current
// window is spent.
func (g *Generator) Next(ctx context.Context) (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.exhausted || g.next > g.max {
		start, end, err := g.alloc.Allocate(ctx, g.batchSize)
		if err != nil {
			return 0, err
		}
		g.next, g.max, g.exhausted = start, end, false
	}

	v := g.next
	g.next++
	return v, nil
}
