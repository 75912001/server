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
	repairedRecord, found, err := r.GetAccountUserRecord(ctx, account)
	if err != nil {
		t.Fatalf("GetAccountUserRecord repair failed: %v", err)
	}
	if !found || repairedRecord.GetUid() != wantUID || repairedRecord.GetAccount() != account {
		t.Fatalf("repaired record = %#v, found = %v", repairedRecord, found)
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

	if err := r.SetUserSessionRecord(ctx, uid, map[string]string{
		userSessionFieldGatewayKey: "gateway-a",
		userSessionFieldOnlineKey:  "online-a",
	}); err != nil {
		t.Fatalf("SetUserSessionRecord failed: %v", err)
	}
	values, err := r.GetUserSessionRecord(ctx, uid, []string{
		userSessionFieldGatewayKey,
		userSessionFieldOnlineKey,
		userSessionFieldLoginTime,
	})
	if err != nil {
		t.Fatalf("GetUserSessionRecord failed: %v", err)
	}
	if len(values) != 2 || values[userSessionFieldGatewayKey] != "gateway-a" || values[userSessionFieldOnlineKey] != "online-a" {
		t.Fatalf("session values = %#v", values)
	}

	cleanupRedisKeys(t, r, ctx, sessionKey)
	emptyExpected := map[string]string{
		userSessionFieldGatewayKey:  "",
		userSessionFieldOnlineKey:   "",
		userSessionFieldUserSession: "",
	}
	oldIdentity := map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldOnlineKey:   "online-1",
		userSessionFieldUserSession: "user-session-1",
	}
	oldRecords := map[string]string{
		userSessionFieldGatewayKey:     "gateway-1",
		userSessionFieldOnlineKey:      "online-1",
		userSessionFieldUserSession:    "user-session-1",
		userSessionFieldGatewaySession: "gateway-session-1",
		userSessionFieldLoginTime:      "111",
	}
	newIdentity := map[string]string{
		userSessionFieldGatewayKey:  "gateway-2",
		userSessionFieldOnlineKey:   "online-2",
		userSessionFieldUserSession: "user-session-2",
	}
	newRecords := map[string]string{
		userSessionFieldGatewayKey:     "gateway-2",
		userSessionFieldOnlineKey:      "online-2",
		userSessionFieldUserSession:    "user-session-2",
		userSessionFieldGatewaySession: "gateway-session-2",
		userSessionFieldLoginTime:      "222",
	}

	replaced, err := r.ReplaceUserSessionRecord(ctx, uid, emptyExpected, oldRecords, 30)
	if err != nil || !replaced {
		t.Fatalf("initial ReplaceUserSessionRecord = %v, %v; want true, nil", replaced, err)
	}

	replaced, err = r.ReplaceUserSessionRecord(ctx, uid, oldIdentity, newRecords, 30)
	if err != nil || !replaced {
		t.Fatalf("replace with old identity = %v, %v; want true, nil", replaced, err)
	}

	deleted, err := r.DelUserSessionRecord(ctx, uid, oldIdentity)
	if err != nil || deleted {
		t.Fatalf("stale DelUserSessionRecord = %v, %v; want false, nil", deleted, err)
	}

	expired, err := r.SetUserSessionExpire(ctx, uid, time.Minute, oldIdentity)
	if err != nil || expired {
		t.Fatalf("stale SetUserSessionExpire = %v, %v; want false, nil", expired, err)
	}

	expired, err = r.SetUserSessionExpire(ctx, uid, time.Minute, newIdentity)
	if err != nil || !expired {
		t.Fatalf("SetUserSessionExpire = %v, %v; want true, nil", expired, err)
	}
	ttl, err := r.client.TTL(ctx, sessionKey).Result()
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("TTL = %v, want positive", ttl)
	}

	deleted, err = r.DelUserSessionRecord(ctx, uid, newIdentity)
	if err != nil || !deleted {
		t.Fatalf("DelUserSessionRecord = %v, %v; want true, nil", deleted, err)
	}
}
