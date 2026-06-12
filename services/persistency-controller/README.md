# persistency-controller

Sole owner of durable storage. Postgres is the source of truth; Redis is a
cache-aside layer with a TTL in front of it. No other service touches the
database or cache directly. Schema migrations run automatically at startup.

## API

gRPC `persistency.v1.PersistencyService`:

- `CreateMapping(code, long_url, expires_at?) -> Mapping` — `INSERT ... ON CONFLICT DO NOTHING`; a collision returns `AlreadyExists`.
- `GetMapping(code) -> Mapping` — cache-first; falls back to Postgres on a miss or a cache failure.

## Configuration

| Env | Default | Description |
| --- | --- | --- |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `OPS_ADDR` | `:9091` | health/readiness/metrics address |
| `POSTGRES_DSN` | `postgres://urlshortener:urlshortener@postgres:5432/urlshortener?sslmode=disable` | Postgres connection string |
| `REDIS_ADDR` | `redis:6379` | Redis cache address |
| `CACHE_TTL` | `1h` | cache entry time-to-live |
| `OTLP_ENDPOINT` | _(empty)_ | OTLP/gRPC trace collector |

## Endpoints

`/healthz`, `/readyz`, `/metrics` on `OPS_ADDR`.
