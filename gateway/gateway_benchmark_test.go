package main

import (
	"testing"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func BenchmarkGrpcErrorToResultCode(b *testing.B) {
	err := grpcstatus.Error(grpccodes.Unavailable, "unavailable")

	for b.Loop() {
		_ = grpcErrorToResultCode(err)
	}
}
