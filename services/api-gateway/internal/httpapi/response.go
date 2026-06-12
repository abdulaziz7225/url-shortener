// Package httpapi is the gateway's REST/redirect edge: handlers, middleware,
// and the mapping of internal gRPC errors to HTTP responses.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/amusaev/url-shortener/libs/grpcutil"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", slog.String("error", err.Error()))
	}
}

// writeError converts a gRPC error from a downstream service into the gateway's
// uniform JSON error body and HTTP status.
func writeError(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, err error) {
	httpStatus := grpcutil.HTTPStatusFromGRPC(err)
	writeJSON(ctx, w, logger, httpStatus, errorResponse{Error: errorBody{
		Code:    codeForStatus(httpStatus),
		Message: grpcutil.PublicMessage(err),
	}})
}

// writeClientError emits a 4xx error originating at the gateway itself.
func writeClientError(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, httpStatus int, message string) {
	writeJSON(ctx, w, logger, httpStatus, errorResponse{Error: errorBody{
		Code:    codeForStatus(httpStatus),
		Message: message,
	}})
}

func codeForStatus(httpStatus int) string {
	switch httpStatus {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGone:
		return "gone"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "unavailable"
	case http.StatusGatewayTimeout:
		return "timeout"
	default:
		return "internal"
	}
}
