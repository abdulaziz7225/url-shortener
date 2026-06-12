package grpcutil

import (
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHTTPStatusFromGRPC(t *testing.T) {
	cases := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, http.StatusOK},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.NotFound, http.StatusNotFound},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.FailedPrecondition, http.StatusGone},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unknown, http.StatusInternalServerError},
	}
	for _, c := range cases {
		err := status.Error(c.code, "x")
		if got := HTTPStatusFromGRPC(err); got != c.want {
			t.Errorf("code %v: got %d, want %d", c.code, got, c.want)
		}
	}
}

func TestHTTPStatusNilIsOK(t *testing.T) {
	if got := HTTPStatusFromGRPC(nil); got != http.StatusOK {
		t.Errorf("nil error: got %d, want 200", got)
	}
}

func TestPublicMessageMasksInfraErrors(t *testing.T) {
	if msg := PublicMessage(status.Error(codes.Internal, "db connection string leaked")); msg != "internal server error" {
		t.Errorf("infra message leaked: %q", msg)
	}
	if msg := PublicMessage(status.Error(codes.NotFound, "code not found")); msg != "code not found" {
		t.Errorf("business message altered: %q", msg)
	}
	if msg := PublicMessage(errors.New("plain")); msg != "internal server error" {
		t.Errorf("non-status error: %q", msg)
	}
}
