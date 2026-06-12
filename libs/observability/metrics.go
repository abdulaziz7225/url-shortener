package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcServerHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total",
		Help: "Total gRPC requests handled by the server, by method and status code.",
	}, []string{"service", "method", "code"})

	grpcServerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_handling_seconds",
		Help:    "gRPC server handling latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method"})

	grpcClientHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_handled_total",
		Help: "Total gRPC requests issued by the client, by method and status code.",
	}, []string{"service", "method", "code"})

	grpcClientDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_handling_seconds",
		Help:    "gRPC client call latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method"})

	httpHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests handled, by route and status code.",
	}, []string{"service", "method", "route", "code"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "route"})
)

// MetricsUnaryServerInterceptor records per-method count and latency for the
// gRPC server.
func MetricsUnaryServerInterceptor(service string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		grpcServerHandled.WithLabelValues(service, info.FullMethod, status.Code(err).String()).Inc()
		grpcServerDuration.WithLabelValues(service, info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// MetricsUnaryClientInterceptor records per-method count and latency for gRPC
// client calls.
func MetricsUnaryClientInterceptor(service string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		grpcClientHandled.WithLabelValues(service, method, status.Code(err).String()).Inc()
		grpcClientDuration.WithLabelValues(service, method).Observe(time.Since(start).Seconds())
		return err
	}
}

// InstrumentHTTP wraps an HTTP handler with count/latency metrics under a fixed
// route label, keeping metric cardinality bounded regardless of the URL path.
func InstrumentHTTP(service, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		httpHandled.WithLabelValues(service, r.Method, route, strconv.Itoa(rec.status)).Inc()
		httpDuration.WithLabelValues(service, r.Method, route).Observe(time.Since(start).Seconds())
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}
