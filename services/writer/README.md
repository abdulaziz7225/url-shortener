# writer

Creates short-code mappings. It validates input, generates codes by
base62-encoding values drawn from the counter service in batches (most requests
need no counter round trip), and persists through the persistency-controller.

## API

gRPC `writer.v1.WriterService`:

- `Shorten(long_url, custom_alias?, expires_at?) -> {code, short_url, long_url, expires_at}`

Behaviour:

- Invalid URL / alias / past expiry -> `InvalidArgument`.
- Taken custom alias -> `AlreadyExists`.
- A generated code that collides with an existing alias is retried with the next counter value.

## Configuration

| Env | Default | Description |
| --- | --- | --- |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `OPS_ADDR` | `:9091` | health/readiness/metrics address |
| `COUNTER_ADDR` | `counter:9090` | counter service address |
| `PERSISTENCY_ADDR` | `persistency-controller:9090` | persistency-controller address |
| `BATCH_SIZE` | `1000` | counter values fetched per allocation |
| `SHORT_URL_BASE` | `http://localhost:8080` | origin used to build returned short URLs |
| `DOWNSTREAM_TIMEOUT` | `2s` | per-call timeout for downstream gRPC |
| `OTLP_ENDPOINT` | _(empty)_ | OTLP/gRPC trace collector |

## Endpoints

`/healthz`, `/readyz`, `/metrics` on `OPS_ADDR`.
