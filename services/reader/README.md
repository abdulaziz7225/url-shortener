# reader

Resolves short codes on the hot path. It reads cache-first through the
persistency-controller and enforces expiry at read time. Reads are idempotent,
so the downstream call is retried on infrastructure failures.

## API

gRPC `reader.v1.ReaderService`:

- `Resolve(code) -> {long_url, expires_at}`

Behaviour:

- Unknown code -> `NotFound` (HTTP 404).
- Expired code -> `FailedPrecondition` (HTTP 410).

## Configuration

| Env | Default | Description |
| --- | --- | --- |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `OPS_ADDR` | `:9091` | health/readiness/metrics address |
| `PERSISTENCY_ADDR` | `persistency-controller:9090` | persistency-controller address |
| `DOWNSTREAM_TIMEOUT` | `2s` | per-call timeout for downstream gRPC |
| `RETRY_ATTEMPTS` | `3` | retry budget for idempotent reads |
| `RETRY_BACKOFF` | `20ms` | linear backoff base between retries |
| `OTLP_ENDPOINT` | _(empty)_ | OTLP/gRPC trace collector |

## Endpoints

`/healthz`, `/readyz`, `/metrics` on `OPS_ADDR`.
