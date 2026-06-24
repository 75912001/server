package main

import (
	"fmt"
	"testing"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func resetTestOnlineAccountMgr(t *testing.T) {
	t.Helper()

	old := GAccountMgr
	GAccountMgr = &AccountMgr{
		accounts: xmap.NewMapMutexMgr[uint64, *Account](),
	}

	t.Cleanup(func() {
		GAccountMgr.accounts.Foreach(func(uid uint64, account *Account) bool {
			if account != nil {
				account.Stop()
			}
			return true
		})
		GAccountMgr = old
	})
}

func setupTestOnlineGRPCServer(t *testing.T) *onlineGRPCServer {
	t.Helper()

	setupFakeCacheGetAccountRecord(t, func(uid uint64) (*pb.AccountRecord, error) {
		return &pb.AccountRecord{
			Uid:                      uid,
			Account:                  fmt.Sprintf("robot.%d", uid),
			AccountCreateTimestampMs: 111,
		}, nil
	})
	return &onlineGRPCServer{}
}

func setupFakeCacheGetAccountRecord(t *testing.T, getAccountRecord func(uid uint64) (*pb.AccountRecord, error)) *fakeCacheService {
	t.Helper()

	return setupFakeCacheServer(t, func(cache *fakeCacheService) {
		cache.getAccountRecord = getAccountRecord
	})
}

func setFakeCacheGetAccountRecord(cache *fakeCacheService, getAccountRecord func(uid uint64) (*pb.AccountRecord, error)) {
	cache.getAccountRecord = getAccountRecord
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
