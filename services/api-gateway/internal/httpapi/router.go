package httpapi

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/amusaev/url-shortener/libs/observability"
)

const serviceName = "api-gateway"

// NewRouter assembles the gateway's HTTP handler: routed endpoints wrapped with
// per-route metrics, the middleware chain, and OpenTelemetry HTTP tracing.
func NewRouter(h *Handlers, limiter *IPRateLimiter, corsOrigin string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Root)
	mux.HandleFunc("POST /api/v1/urls", observability.InstrumentHTTP(serviceName, "create_url", h.Create))
	mux.HandleFunc("GET /api/v1/urls/{code}", observability.InstrumentHTTP(serviceName, "get_url", h.Metadata))
	mux.HandleFunc("GET /{code}", observability.InstrumentHTTP(serviceName, "redirect", h.Redirect))

	handler := chain(mux,
		Recover(logger),
		CORS(corsOrigin),
		RateLimit(limiter, logger),
		RequestLog(logger),
	)

	return otelhttp.NewHandler(handler, "api-gateway")
}
