// Package store owns the Postgres source of truth for URL mappings, including
// startup migrations.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amusaev/url-shortener/services/persistency-controller/internal/model"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Postgres is the durable repository for mappings.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres connects a traced pool and verifies connectivity.
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases the pool.
func (p *Postgres) Close() { p.pool.Close() }

// Migrate applies any embedded migrations that have not yet run.
func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := p.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := p.pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := p.pool.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1)`, name,
		); err != nil {
			return err
		}
	}
	return nil
}

// Create inserts a mapping. It returns (mapping, true) when a new row was
// written, or (nil, false) when the code already exists — the structural
// uniqueness safety net.
func (p *Postgres) Create(ctx context.Context, m model.Mapping) (*model.Mapping, bool, error) {
	row := p.pool.QueryRow(ctx,
		`INSERT INTO urls (code, long_url, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (code) DO NOTHING
		 RETURNING created_at`,
		m.Code, m.LongURL, m.ExpiresAt,
	)
	if err := row.Scan(&m.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &m, true, nil
}

// Get loads a mapping by code, returning model.ErrNotFound when absent.
func (p *Postgres) Get(ctx context.Context, code string) (*model.Mapping, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT code, long_url, expires_at, created_at FROM urls WHERE code = $1`, code)

	var m model.Mapping
	if err := row.Scan(&m.Code, &m.LongURL, &m.ExpiresAt, &m.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}
