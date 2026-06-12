// Package cache is the Redis cache-aside layer in front of Postgres.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/amusaev/url-shortener/services/persistency-controller/internal/model"
)

// Cache stores mappings in Redis with a TTL.
type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

// New builds a cache with the given TTL.
func New(rdb *redis.Client, ttl time.Duration) *Cache {
	return &Cache{rdb: rdb, ttl: ttl}
}

func key(code string) string { return "url:" + code }

type entry struct {
	LongURL   string     `json:"long_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Get returns the cached mapping. The bool is false on a cache miss; a non-nil
// error signals a cache failure (the caller falls back to the database).
func (c *Cache) Get(ctx context.Context, code string) (*model.Mapping, bool, error) {
	raw, err := c.rdb.Get(ctx, key(code)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var e entry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return nil, false, err
	}
	return &model.Mapping{
		Code:      code,
		LongURL:   e.LongURL,
		ExpiresAt: e.ExpiresAt,
		CreatedAt: e.CreatedAt,
	}, true, nil
}

// Set writes a mapping into the cache with the configured TTL.
func (c *Cache) Set(ctx context.Context, m model.Mapping) error {
	raw, err := json.Marshal(entry{
		LongURL:   m.LongURL,
		ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	})
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key(m.Code), raw, c.ttl).Err()
}
