package observability

import (
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Readiness is a flag a service flips to true once its dependencies are wired.
type Readiness struct {
	ready atomic.Bool
}

// SetReady marks the service ready (or not) for the /readyz probe.
func (r *Readiness) SetReady(v bool) { r.ready.Store(v) }

// IsReady reports the current readiness state.
func (r *Readiness) IsReady() bool { return r.ready.Load() }

// NewOpsServer builds the side HTTP server exposing liveness (/healthz),
// readiness (/readyz), and Prometheus metrics (/metrics). It is used by every
// service and is what Docker healthchecks probe.
func NewOpsServer(addr string, ready *Readiness) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready == nil || ready.IsReady() {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ready")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "not ready")
	})
	mux.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
