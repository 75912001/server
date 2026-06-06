package main

import (
	"fmt"
	"testing"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func resetTestOnlineUserMgr(t *testing.T) {
	t.Helper()

	old := GUserMgr
	GUserMgr = &UserMgr{
		users: xmap.NewMapMutexMgr[uint64, *User](),
	}

	t.Cleanup(func() {
		GUserMgr.users.Foreach(func(uid uint64, user *User) bool {
			if user != nil {
				user.Stop()
			}
			return true
		})
		GUserMgr = old
	})
}

func newTestOnlineGRPCServer() *onlineGRPCServer {
	return &onlineGRPCServer{
		cacheGetUserRecord: func(uid uint64) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				Uid:                 uid,
				Account:             fmt.Sprintf("robot.%d", uid),
				AccountCreateTimeMs: 111,
			}, nil
		},
	}
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
