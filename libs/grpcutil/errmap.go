// Package grpcutil provides shared gRPC client/server wiring and the canonical
// mapping between internal gRPC status codes and outward-facing HTTP statuses.
package grpcutil

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPStatusFromGRPC maps a gRPC error to the HTTP status code the gateway
// returns to clients. A nil error maps to 200.
func HTTPStatusFromGRPC(err error) int {
	return httpStatusForCode(status.Code(err))
}

func httpStatusForCode(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.FailedPrecondition:
		return http.StatusGone
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// PublicMessage returns a client-safe message for an error. Business errors
// carry their gRPC message through; infrastructure failures are masked so that
// internal details never leak past the gateway.
func PublicMessage(err error) string {
	if err == nil {
		return ""
	}
	st := status.Convert(err)
	switch st.Code() {
	case codes.InvalidArgument, codes.OutOfRange, codes.NotFound,
		codes.AlreadyExists, codes.FailedPrecondition, codes.ResourceExhausted,
		codes.Unauthenticated, codes.PermissionDenied, codes.Unimplemented:
		return st.Message()
	case codes.DeadlineExceeded:
		return "upstream service timed out"
	case codes.Unavailable:
		return "service temporarily unavailable"
	default:
		return "internal server error"
	}
}
