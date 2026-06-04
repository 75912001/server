package main

import (
	"testing"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type testCleanup interface {
	Helper()
	Cleanup(func())
}

func setTestCacheConfig(t testCleanup) {
	t.Helper()

	oldUserRecord := GCfgCustomRedisKeyFormatUserRecord
	oldUserSession := GCfgCustomRedisKeyFormatUserSession
	oldAccountToken := GCfgCustomRedisKeyFormatAccountToken
	oldAccountUID := GCfgCustomRedisKeyFormatAccountUID
	oldAccountLock := GCfgCustomRedisKeyFormatAccountLock
	oldLockDuration := GCfgCustomRedisAccountCreateLockDuration
	oldGroupID := GCfgBaseGroupID

	GCfgCustomRedisKeyFormatUserRecord = "user:{%v}:record"
	GCfgCustomRedisKeyFormatUserSession = "user:{%v}:session"
	GCfgCustomRedisKeyFormatAccountToken = "account:{%v}:token"
	GCfgCustomRedisKeyFormatAccountUID = "account:{%v}:uid"
	GCfgCustomRedisKeyFormatAccountLock = "account:{%v}:lock"
	GCfgCustomRedisAccountCreateLockDuration = 5 * time.Second
	GCfgBaseGroupID = 1

	t.Cleanup(func() {
		GCfgCustomRedisKeyFormatUserRecord = oldUserRecord
		GCfgCustomRedisKeyFormatUserSession = oldUserSession
		GCfgCustomRedisKeyFormatAccountToken = oldAccountToken
		GCfgCustomRedisKeyFormatAccountUID = oldAccountUID
		GCfgCustomRedisKeyFormatAccountLock = oldAccountLock
		GCfgCustomRedisAccountCreateLockDuration = oldLockDuration
		GCfgBaseGroupID = oldGroupID
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
