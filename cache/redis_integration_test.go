//go:build integration

package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"server/common"
	pb "server/proto/pb"
)

type integrationTB interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
	Skip(...any)
}

func newIntegrationRedis(t integrationTB) (*Redis, context.Context) {
	t.Helper()

	addrsValue := os.Getenv("CACHE_TEST_REDIS_ADDRS")
	if addrsValue == "" {
		t.Skip("set CACHE_TEST_REDIS_ADDRS to run Redis integration tests")
	}

	addrs := strings.Split(addrsValue, ",")
	for i := range addrs {
		addrs[i] = strings.TrimSpace(addrs[i])
	}

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs,
		Password: os.Getenv("CACHE_TEST_REDIS_PASSWORD"),
	})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping failed: %v", err)
	}

	setTestCacheConfig(t)
	GCfgBaseGroupID = integrationGroupID(t)
	return &Redis{client: client}, ctx
}

func integrationGroupID(t integrationTB) uint32 {
	t.Helper()

	value := os.Getenv("CACHE_TEST_GROUP_ID")
	if value == "" {
		return 9999
	}
	groupID, err := strconv.ParseUint(value, 10, 32)
	if err != nil || groupID == 0 {
		t.Fatalf("invalid CACHE_TEST_GROUP_ID: %q", value)
	}
	return uint32(groupID)
}

func integrationSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func cleanupRedisKeys(t *testing.T, r *Redis, ctx context.Context, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := r.client.Del(ctx, key).Err(); err != nil {
			t.Fatalf("delete redis key %q failed: %v", key, err)
		}
	}
}

func TestIntegrationAccountToken(t *testing.T) {
	r, ctx := newIntegrationRedis(t)

	account := "it-token-" + integrationSuffix()
	token := "token-" + integrationSuffix()
	tokenKey := RedisKeyAccountToken(account)
	t.Cleanup(func() {
		cleanupRedisKeys(t, r, ctx, tokenKey)
	})

	ok, err := r.SetAccountVerifyToken(ctx, account, token, time.Minute)
	if err != nil || !ok {
		t.Fatalf("SetAccountVerifyToken first = %v, %v; want true, nil", ok, err)
	}

	ok, err = r.SetAccountVerifyToken(ctx, account, token, time.Minute)
	if err != nil || ok {
		t.Fatalf("SetAccountVerifyToken duplicate = %v, %v; want false, nil", ok, err)
	}

	ok, err = r.UseAccountVerifyToken(ctx, account, "bad-token")
	if err != nil || ok {
		t.Fatalf("UseAccountVerifyToken wrong token = %v, %v; want false, nil", ok, err)
	}

	ok, err = r.UseAccountVerifyToken(ctx, account, token)
	if err != nil || !ok {
		t.Fatalf("UseAccountVerifyToken correct token = %v, %v; want true, nil", ok, err)
	}

	exists, err := r.client.Exists(ctx, tokenKey).Result()
	if err != nil {
		t.Fatalf("exists token key failed: %v", err)
	}
	if exists != 0 {
		t.Fatalf("token key exists = %d, want 0", exists)
	}

	ok, err = r.UseAccountVerifyToken(ctx, account, token)
	if err != nil || ok {
		t.Fatalf("UseAccountVerifyToken consumed token = %v, %v; want false, nil", ok, err)
	}
}

func TestIntegrationEnsureAccount(t *testing.T) {
	r, ctx := newIntegrationRedis(t)

	account := "it-account-" + integrationSuffix()
	sequenceKey := RedisKeyUserUIDSequence(GCfgBaseGroupID)
	accountUIDKey := RedisKeyAccountUID(account)
	accountLockKey := RedisKeyAccountLock(account)
	cleanupRedisKeys(t, r, ctx, sequenceKey, accountUIDKey, accountLockKey)

	userRecord, created, err := r.EnsureAccount(ctx, account)
	if err != nil {
		t.Fatalf("EnsureAccount create failed: %v", err)
	}
	if !created {
		t.Fatal("EnsureAccount created = false, want true")
	}
	wantUID := common.GroupUIDStart(GCfgBaseGroupID)
	if userRecord.GetUid() != wantUID {
		t.Fatalf("uid = %d, want %d", userRecord.GetUid(), wantUID)
	}

	userRecordAgain, createdAgain, err := r.EnsureAccount(ctx, account)
	if err != nil {
		t.Fatalf("EnsureAccount existing failed: %v", err)
	}
	if createdAgain {
		t.Fatal("EnsureAccount existing created = true, want false")
	}
	if userRecordAgain.GetUid() != wantUID {
		t.Fatalf("existing uid = %d, want %d", userRecordAgain.GetUid(), wantUID)
	}

	userRecordKey := RedisKeyUserRecord(wantUID)
	cleanupRedisKeys(t, r, ctx, userRecordKey)
	if _, found, err := r.GetAccountUserRecord(ctx, account); err == nil || !found {
		t.Fatalf("GetAccountUserRecord missing record err = %v, found = %v; want error and found", err, found)
	}

	now := time.Now().UnixMilli()
	if err := r.SetUserRecord(ctx, wantUID, &pb.UserRecord{Uid: wantUID + 1, Account: account, AccountCreateTimeMs: now}); err != nil {
		t.Fatalf("set mismatched uid user record failed: %v", err)
	}
	if _, found, err := r.GetAccountUserRecord(ctx, account); err == nil || !found {
		t.Fatalf("GetAccountUserRecord mismatched uid err = %v, found = %v; want error and found", err, found)
	}

	if err := r.SetUserRecord(ctx, wantUID, &pb.UserRecord{Uid: wantUID, Account: account + "-other", AccountCreateTimeMs: now}); err != nil {
		t.Fatalf("set mismatched account user record failed: %v", err)
	}
	if _, found, err := r.GetAccountUserRecord(ctx, account); err == nil || !found {
		t.Fatalf("GetAccountUserRecord mismatched account err = %v, found = %v; want error and found", err, found)
	}

	if err := r.SetUserRecord(ctx, wantUID, &pb.UserRecord{Uid: wantUID, Account: account}); err != nil {
		t.Fatalf("set empty account create time user record failed: %v", err)
	}
	if _, found, err := r.GetAccountUserRecord(ctx, account); err == nil || !found {
		t.Fatalf("GetAccountUserRecord empty account create time err = %v, found = %v; want error and found", err, found)
	}

	if err := r.SetUserRecord(ctx, wantUID, &pb.UserRecord{Uid: wantUID, Account: account, AccountCreateTimeMs: now, UserCreateTimeMs: 0}); err != nil {
		t.Fatalf("set valid user record with empty user create time failed: %v", err)
	}
	validRecord, found, err := r.GetAccountUserRecord(ctx, account)
	if err != nil || !found {
		t.Fatalf("GetAccountUserRecord valid empty user create time = %#v, %v, %v; want record, true, nil", validRecord, found, err)
	}

	cleanupRedisKeys(t, r, ctx, sequenceKey, accountUIDKey, accountLockKey, userRecordKey)
}

func TestIntegrationEnsureAccountConcurrent(t *testing.T) {
	r, ctx := newIntegrationRedis(t)

	account := "it-concurrent-" + integrationSuffix()
	sequenceKey := RedisKeyUserUIDSequence(GCfgBaseGroupID)
	accountUIDKey := RedisKeyAccountUID(account)
	accountLockKey := RedisKeyAccountLock(account)
	cleanupRedisKeys(t, r, ctx, sequenceKey, accountUIDKey, accountLockKey)

	const workers = 8
	uids := make(chan uint64, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, _, err := r.EnsureAccount(ctx, account)
			if err != nil {
				errs <- err
				return
			}
			uids <- record.GetUid()
		}()
	}
	wg.Wait()
	close(uids)
	close(errs)

	for err := range errs {
		t.Fatalf("EnsureAccount concurrent failed: %v", err)
	}
	var firstUID uint64
	for uid := range uids {
		if firstUID == 0 {
			firstUID = uid
			continue
		}
		if uid != firstUID {
			t.Fatalf("concurrent uid = %d, want %d", uid, firstUID)
		}
	}

	cleanupRedisKeys(t, r, ctx, sequenceKey, accountUIDKey, accountLockKey, RedisKeyUserRecord(firstUID))
}

func TestIntegrationUserSessionCAS(t *testing.T) {
	r, ctx := newIntegrationRedis(t)

	uid := common.GroupUIDStart(GCfgBaseGroupID) + 100
	sessionKey := RedisKeyUserSession(uid)
	cleanupRedisKeys(t, r, ctx, sessionKey)
	t.Cleanup(func() {
		cleanupRedisKeys(t, r, ctx, sessionKey)
	})

	oldUserSession := "user-session-1"
	oldRecords := map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldUserSession: "user-session-1",
		userSessionFieldLoginTime:   "111",
		userSessionFieldOnlineKey:   "online-1",
	}
	newUserSession := "user-session-2"
	newRecords := map[string]string{
		userSessionFieldGatewayKey:  "gateway-2",
		userSessionFieldUserSession: "user-session-2",
		userSessionFieldLoginTime:   "222",
		userSessionFieldOnlineKey:   "online-2",
	}

	begun, err := r.BeginUserSessionCAS(ctx, uid, "", oldRecords, 30)
	if err != nil || !begun {
		t.Fatalf("initial BeginUserSessionCAS = %v, %v; want true, nil", begun, err)
	}

	values, err := r.GetUserSession(ctx, uid)
	if err != nil {
		t.Fatalf("GetUserSession failed: %v", err)
	}
	if values[userSessionFieldGatewayKey] != "gateway-1" || values[userSessionFieldUserSession] != "user-session-1" || values[userSessionFieldOnlineKey] != "online-1" {
		t.Fatalf("session values = %#v", values)
	}

	begun, err = r.BeginUserSessionCAS(ctx, uid, oldUserSession, newRecords, 30)
	if err != nil || !begun {
		t.Fatalf("replace with old identity = %v, %v; want true, nil", begun, err)
	}

	deleted, err := r.EndUserSessionCAS(ctx, uid, oldUserSession)
	if err != nil || deleted {
		t.Fatalf("stale EndUserSessionCAS = %v, %v; want false, nil", deleted, err)
	}

	refreshed, err := r.RefreshUserSessionCAS(ctx, uid, oldUserSession, 30)
	if err != nil || refreshed {
		t.Fatalf("stale RefreshUserSessionCAS = %v, %v; want false, nil", refreshed, err)
	}

	refreshed, err = r.RefreshUserSessionCAS(ctx, uid, newUserSession, 30)
	if err != nil || !refreshed {
		t.Fatalf("RefreshUserSessionCAS = %v, %v; want true, nil", refreshed, err)
	}
	ttl, err := r.client.TTL(ctx, sessionKey).Result()
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("TTL = %v, want positive", ttl)
	}

	deleted, err = r.EndUserSessionCAS(ctx, uid, newUserSession)
	if err != nil || !deleted {
		t.Fatalf("EndUserSessionCAS = %v, %v; want true, nil", deleted, err)
	}
}
