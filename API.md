# API

Base URL: `http://localhost:8080`

All request and response bodies are JSON. Timestamps are RFC 3339 in UTC
(e.g. `2030-01-01T00:00:00Z`). A [Postman collection](url-shortener.postman_collection.json)
is included.

## Create a short URL

`POST /api/v1/urls`

Request body:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `long_url` | string | yes | Absolute `http(s)` URL to shorten. |
| `custom_alias` | string | no | Desired alias (`[A-Za-z0-9_-]{1,64}`). |
| `expires_at` | string (RFC 3339) | no | Future expiration time. |

```bash
curl -X POST http://localhost:8080/api/v1/urls \
  -H 'Content-Type: application/json' \
  -d '{"long_url":"https://example.com/path","custom_alias":"promo","expires_at":"2030-01-01T00:00:00Z"}'
```

`201 Created`:

```json
{
  "code": "promo",
  "short_url": "http://localhost:8080/promo",
  "long_url": "https://example.com/path",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

Errors: `400` invalid URL/alias/expiry, `409` alias already taken.

## Redirect

`GET /{code}`

```bash
curl -i http://localhost:8080/promo
```

`302 Found` with a `Location` header pointing at the long URL.

Errors: `404` unknown code, `410 Gone` expired.

## Get metadata

`GET /api/v1/urls/{code}`

```bash
curl http://localhost:8080/api/v1/urls/promo
```

`200 OK`:

```json
{
  "code": "promo",
  "long_url": "https://example.com/path",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

Errors: `404` unknown code, `410 Gone` expired.

## Error model

Every error shares one shape:

```json
{ "error": { "code": "not_found", "message": "code \"abc\" not found" } }
```

| HTTP | `code` | When |
| --- | --- | --- |
| 400 | `invalid_request` | Validation failed. |
| 404 | `not_found` | Unknown short code. |
| 409 | `conflict` | Custom alias already exists. |
| 410 | `gone` | Link has expired. |
| 429 | `rate_limited` | Per-client rate limit exceeded. |
| 503 | `unavailable` | A downstream dependency is unavailable (circuit open). |
| 504 | `timeout` | A downstream call timed out. |
| 500 | `internal` | Unexpected server error. |

## Rate limiting

The gateway applies a per-client-IP token bucket (default 50 req/s, burst 100).
Requests over budget receive `429` with the standard error body.

## Operational endpoints

Each service exposes `/healthz`, `/readyz`, and `/metrics` on its ops port
(the gateway's is `:8081`). These are used by Docker healthchecks and Prometheus.
