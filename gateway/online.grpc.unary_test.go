package main

import (
	"errors"
	"testing"

	xerror "github.com/75912001/xlib/error"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestGrpcErrorToResultCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want uint32
	}{
		{name: "plain error", err: errors.New("plain error"), want: xerror.Fail.Code()},
		{name: "canceled", err: grpcstatus.Error(grpccodes.Canceled, "canceled"), want: xerror.Cancelled.Code()},
		{name: "unknown", err: grpcstatus.Error(grpccodes.Unknown, "unknown"), want: xerror.Unknown.Code()},
		{name: "invalid argument", err: grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument"), want: xerror.InvalidArgument.Code()},
		{name: "deadline exceeded", err: grpcstatus.Error(grpccodes.DeadlineExceeded, "deadline exceeded"), want: xerror.DeadlineExceeded.Code()},
		{name: "not found", err: grpcstatus.Error(grpccodes.NotFound, "not found"), want: xerror.NotFound.Code()},
		{name: "already exists", err: grpcstatus.Error(grpccodes.AlreadyExists, "already exists"), want: xerror.AlreadyExists.Code()},
		{name: "permission denied", err: grpcstatus.Error(grpccodes.PermissionDenied, "permission denied"), want: xerror.PermissionDenied.Code()},
		{name: "resource exhausted", err: grpcstatus.Error(grpccodes.ResourceExhausted, "resource exhausted"), want: xerror.ResourceExhausted.Code()},
		{name: "failed precondition", err: grpcstatus.Error(grpccodes.FailedPrecondition, "failed precondition"), want: xerror.FailedPrecondition.Code()},
		{name: "aborted", err: grpcstatus.Error(grpccodes.Aborted, "aborted"), want: xerror.Aborted.Code()},
		{name: "out of range", err: grpcstatus.Error(grpccodes.OutOfRange, "out of range"), want: xerror.OutOfRange.Code()},
		{name: "unimplemented", err: grpcstatus.Error(grpccodes.Unimplemented, "unimplemented"), want: xerror.Unimplemented.Code()},
		{name: "internal", err: grpcstatus.Error(grpccodes.Internal, "internal"), want: xerror.Internal.Code()},
		{name: "unavailable", err: grpcstatus.Error(grpccodes.Unavailable, "unavailable"), want: xerror.Unavailable.Code()},
		{name: "data loss", err: grpcstatus.Error(grpccodes.DataLoss, "data loss"), want: xerror.DataLoss.Code()},
		{name: "unauthenticated", err: grpcstatus.Error(grpccodes.Unauthenticated, "unauthenticated"), want: xerror.Unauthenticated.Code()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := grpcErrorToResultCode(tt.err); got != tt.want {
				t.Fatalf("result code = %d, want %d", got, tt.want)
			}
		})
	}
}
