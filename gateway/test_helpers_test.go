package main

import (
	"testing"

	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type testCleanup interface {
	Helper()
	Cleanup(func())
}

func resetTestGatewayUserMgr(t testCleanup) {
	t.Helper()

	old := GUserMgr
	GUserMgr = &UserMgr{
		byRemote: xmap.NewMapMutexMgr[xnetcommon.IRemote, *User](),
		byUID:    xmap.NewMapMutexMgr[uint64, *User](),
	}

	t.Cleanup(func() {
		GUserMgr = old
	})
}

func requireStatusCode(t *testing.T, err error, want grpccodes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected gRPC status %v, got nil", want)
	}
	if got := grpcstatus.Code(err); got != want {
		t.Fatalf("status code = %v, want %v, err: %v", got, want, err)
	}
}
