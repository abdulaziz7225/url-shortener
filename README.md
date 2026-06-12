# URL Shortener

A production-style, Bit.ly-modeled URL shortener built as five Go microservices
with a REST edge, gRPC internals, Postgres + Redis, full observability, and a
React frontend. The entire stack runs with a single command.

## Features

- **Shorten** long URLs into `http://localhost:8080/{code}`.
- **Custom aliases** (optional per request); taken aliases are rejected with `409 Conflict`.
- **Expiration** (optional per request); expired links return `410 Gone`.
- **Redirect** via `302` to the original URL.

Short-code uniqueness is structural — a single global counter hands out disjoint
ranges, base62-encoded by writers — with a database `UNIQUE` constraint as the
final safety net. Reads are cache-first and fall back to Postgres when the cache
degrades.

## Architecture

REST/JSON at the edge; gRPC + Protocol Buffers for all internal calls.

```
                  ┌────────────┐
   browser  ──▶   │  frontend  │  (React + Vite, :3000)
                  └─────┬──────┘
                        │ REST/JSON
                  ┌─────▼───────┐
   curl/Postman ─▶│ api-gateway │  :8080  rate limit · breakers · error mapping
                  └──┬───────┬──┘
              gRPC   │       │   gRPC
            ┌────────▼─┐   ┌─▼─────────┐
            │  writer  │   │  reader   │
            └──┬────┬──┘   └─────┬─────┘
       gRPC    │    │ gRPC       │ gRPC
        ┌──────▼─┐ ┌▼────────────▼──────────┐
        │counter │ │ persistency-controller │
        └───┬────┘ └────┬───────────────┬───┘
            │           │               │
         ┌──▼──┐    ┌───▼────┐     ┌────▼────┐
         │redis│    │postgres│     │  redis  │
         │(seq)│    │(truth) │     │ (cache) │
         └─────┘    └────────┘     └─────────┘
```

| Service | Role |
| --- | --- |
| [api-gateway](services/api-gateway/) | Only public entry point: REST API, redirect, rate limiting, circuit breakers, gRPC→HTTP error mapping. |
| [writer](services/writer/) | Validates input, generates codes from batched counter ranges, persists mappings. |
| [reader](services/reader/) | Hot-path resolution, cache-first, expiry enforced at read time. |
| [counter](services/counter/) | Uniqueness source of truth: disjoint counter ranges via atomic Redis `INCRBY`. |
| [persistency-controller](services/persistency-controller/) | Sole owner of Postgres (truth) and Redis (cache-aside); migrations at startup. |

Every service ships structured JSON logging, OpenTelemetry tracing to Jaeger,
Prometheus metrics, per-call timeouts, circuit breakers that trip only on
infrastructure failures, retries for idempotent reads, health/readiness probes,
and graceful shutdown.

## Quick start

Requirements: Docker + Docker Compose.

```bash
make up      # build and start the whole stack from scratch
make e2e     # run the end-to-end smoke test against it
make down    # stop and remove volumes
```

| Surface | URL |
| --- | --- |
| Frontend | http://localhost:3000 |
| REST API / redirects | http://localhost:8080 |
| Jaeger (traces) | http://localhost:16686 |
| Prometheus | http://localhost:9090 |
| Grafana (dashboard) | http://localhost:3001 |

### Try it

```bash
# create
curl -X POST http://localhost:8080/api/v1/urls \
  -H 'Content-Type: application/json' \
  -d '{"long_url":"https://example.com/some/very/long/path"}'

# follow the redirect
curl -i http://localhost:8080/<code>
```

See [API.md](API.md) for the full API and a Postman collection.

## Development

```bash
make tools   # install protoc plugins and golangci-lint (pinned versions)
make proto   # regenerate gRPC stubs into libs/gen
make build   # compile all Go packages
make test    # run unit tests
make lint    # run golangci-lint
```

Run `make help` to list all targets.

## Observability

- **Traces**: a single request is traced end to end across services. In Jaeger,
  select the `api-gateway` service and open a trace to see spans spanning
  gateway → writer → counter / persistency, or gateway → reader → persistency.
- **Metrics**: Prometheus scrapes every service; Grafana loads a pre-provisioned
  "URL Shortener" dashboard (HTTP and gRPC rates, latencies, and error rates).
- **Logs**: structured JSON on stdout, correlated by `trace_id`.

## Layout

```
libs/                shared code
  proto/  gen/       gRPC contracts and generated stubs
  base62/ grpcutil/ observability/ resilience/ service/ redisx/ config/
services/            the five microservices (each with a Dockerfile and README)
frontend/            React + TypeScript + Vite SPA
deploy/              Prometheus and Grafana provisioning
scripts/e2e.sh       end-to-end smoke test
```

## Testing

- **Unit tests** cover code generation and counter batching, input validation,
  the gRPC↔HTTP error mapping, cache hit/miss/fallback behaviour, and rate
  limiting.
- **End-to-end** (`make e2e`) exercises create, redirect, metadata, alias
  conflict, validation errors, expiry, unknown codes, and rate limiting against
  the live stack.
