# api-gateway

The only public entry point. It exposes the REST/JSON API and the redirect
route, applies per-client rate limiting, wraps each downstream in a circuit
breaker, and maps internal gRPC errors to HTTP status codes with a uniform JSON
error body.

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/urls` | Create a short URL. Body: `{ "long_url", "custom_alias?", "expires_at?" }`. Returns `201`. |
| `GET` | `/api/v1/urls/{code}` | Fetch mapping metadata as JSON. `404`/`410` for unknown/expired. |
| `GET` | `/{code}` | Redirect (`302`) to the long URL. `404` unknown, `410` expired. |
| `GET` | `/` | Liveness JSON. |

`expires_at` is RFC 3339 (e.g. `2030-01-01T00:00:00Z`).

### Error body

```json
{ "error": { "code": "not_found", "message": "code \"abc\" not found" } }
```

### Status mapping

`InvalidArgument→400`, `NotFound→404`, `AlreadyExists→409`, `FailedPrecondition→410`,
`ResourceExhausted/rate-limit→429`, `Unavailable→503`, `DeadlineExceeded→504`, else `500`.

## Configuration

| Env | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | public HTTP listen address |
| `OPS_ADDR` | `:8081` | health/readiness/metrics address |
| `WRITER_ADDR` | `writer:9090` | writer service address |
| `READER_ADDR` | `reader:9090` | reader service address |
| `CORS_ORIGIN` | `*` | allowed CORS origin |
| `RATE_LIMIT_RPS` | `50` | per-IP requests per second |
| `RATE_LIMIT_BURST` | `100` | per-IP burst |
| `DOWNSTREAM_TIMEOUT` | `2s` | per-call timeout for downstream gRPC |
| `OTLP_ENDPOINT` | _(empty)_ | OTLP/gRPC trace collector |

## Endpoints (ops)

`/healthz`, `/readyz`, `/metrics` on `OPS_ADDR`.
