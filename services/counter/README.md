# counter

The uniqueness source of truth. `AllocateRange` hands out disjoint, inclusive
`[start, end]` ranges backed by an atomic Redis `INCRBY`, so horizontally scaled
writers never produce colliding short codes.

## API

gRPC `counter.v1.CounterService`:

- `AllocateRange(batch_size) -> {start, end}`

## Configuration

| Env | Default | Description |
| --- | --- | --- |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `OPS_ADDR` | `:9091` | health/readiness/metrics address |
| `REDIS_ADDR` | `redis:6379` | Redis used for the atomic counter |
| `COUNTER_KEY` | `counter:sequence` | Redis key holding the global counter |
| `OTLP_ENDPOINT` | _(empty)_ | OTLP/gRPC trace collector; tracing off when unset |

## Endpoints

`/healthz`, `/readyz`, `/metrics` on `OPS_ADDR`.
