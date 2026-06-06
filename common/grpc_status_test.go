package common

import (
	"errors"
	"net/http"
	"testing"

	xerror "github.com/75912001/xlib/error"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestGRPCCodeToResultID(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		want uint32
	}{
		{name: "ok", code: codes.OK, want: xerror.Success.Code()},
		{name: "canceled", code: codes.Canceled, want: xerror.Cancelled.Code()},
		{name: "unknown", code: codes.Unknown, want: xerror.Unknown.Code()},
		{name: "invalid argument", code: codes.InvalidArgument, want: xerror.InvalidArgument.Code()},
		{name: "deadline exceeded", code: codes.DeadlineExceeded, want: xerror.DeadlineExceeded.Code()},
		{name: "not found", code: codes.NotFound, want: xerror.NotFound.Code()},
		{name: "already exists", code: codes.AlreadyExists, want: xerror.AlreadyExists.Code()},
		{name: "permission denied", code: codes.PermissionDenied, want: xerror.PermissionDenied.Code()},
		{name: "resource exhausted", code: codes.ResourceExhausted, want: xerror.ResourceExhausted.Code()},
		{name: "failed precondition", code: codes.FailedPrecondition, want: xerror.FailedPrecondition.Code()},
		{name: "aborted", code: codes.Aborted, want: xerror.Aborted.Code()},
		{name: "out of range", code: codes.OutOfRange, want: xerror.OutOfRange.Code()},
		{name: "unimplemented", code: codes.Unimplemented, want: xerror.Unimplemented.Code()},
		{name: "internal", code: codes.Internal, want: xerror.Internal.Code()},
		{name: "unavailable", code: codes.Unavailable, want: xerror.Unavailable.Code()},
		{name: "data loss", code: codes.DataLoss, want: xerror.DataLoss.Code()},
		{name: "unauthenticated", code: codes.Unauthenticated, want: xerror.Unauthenticated.Code()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GRPCCodeToResultID(tt.code); got != tt.want {
				t.Fatalf("result id = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGRPCStatusToResultID(t *testing.T) {
	if got := GRPCStatusToResultID(nil); got != xerror.Success.Code() {
		t.Fatalf("nil result id = %d, want success", got)
	}
	if got := GRPCStatusToResultID(errors.New("plain")); got != xerror.Fail.Code() {
		t.Fatalf("plain error result id = %d, want fail", got)
	}
	if got := GRPCStatusToResultID(grpcstatus.Error(codes.DataLoss, "bad data")); got != xerror.DataLoss.Code() {
		t.Fatalf("data loss result id = %d, want data loss", got)
	}
}

func TestGRPCCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{code: codes.InvalidArgument, want: http.StatusBadRequest},
		{code: codes.Unauthenticated, want: http.StatusUnauthorized},
		{code: codes.PermissionDenied, want: http.StatusForbidden},
		{code: codes.Aborted, want: http.StatusConflict},
		{code: codes.ResourceExhausted, want: http.StatusTooManyRequests},
		{code: codes.Unavailable, want: http.StatusServiceUnavailable},
		{code: codes.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{code: codes.DataLoss, want: http.StatusBadGateway},
	}

	for _, tt := range tests {
		if got := GRPCCodeToHTTPStatus(tt.code); got != tt.want {
			t.Fatalf("http status for %v = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestGRPCStatusToHTTP(t *testing.T) {
	statusCode, message := GRPCStatusToHTTP(grpcstatus.Error(codes.Aborted, "session changed"), "fallback")
	if statusCode != http.StatusConflict || message != "session changed" {
		t.Fatalf("status=%d message=%q", statusCode, message)
	}

	statusCode, message = GRPCStatusToHTTP(errors.New("plain"), "fallback")
	if statusCode != http.StatusServiceUnavailable || message != "fallback" {
		t.Fatalf("plain status=%d message=%q", statusCode, message)
	}
}
