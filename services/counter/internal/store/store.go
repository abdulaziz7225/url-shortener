// Package store backs the counter with an atomic Redis increment.
package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Allocator hands out inclusive [start, end] ranges of counter values.
type Allocator interface {
	Allocate(ctx context.Context, batchSize uint64) (start, end uint64, err error)
}

// RedisAllocator implements Allocator using a single Redis key and INCRBY,
// which is atomic across concurrent callers and therefore yields disjoint
// ranges even with many writers.
type RedisAllocator struct {
	rdb *redis.Client
	key string
}

// NewRedisAllocator returns an allocator over the given Redis key.
func NewRedisAllocator(rdb *redis.Client, key string) *RedisAllocator {
	return &RedisAllocator{rdb: rdb, key: key}
}

// Allocate atomically reserves batchSize values and returns their inclusive
// bounds. The returned end is the new counter value; start is end-batchSize+1.
func (a *RedisAllocator) Allocate(ctx context.Context, batchSize uint64) (uint64, uint64, error) {
	end, err := a.rdb.IncrBy(ctx, a.key, int64(batchSize)).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("incrby %s: %w", a.key, err)
	}
	return uint64(end) - batchSize + 1, uint64(end), nil
}
