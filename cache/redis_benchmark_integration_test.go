//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

func BenchmarkIntegrationSetAccountVerifyToken(b *testing.B) {
	r, ctx := newIntegrationRedisForBenchmark(b)

	for i := 0; i < b.N; i++ {
		account := "bench-token-" + integrationSuffix()
		if ok, err := r.SetAccountVerifyToken(ctx, account, "token", time.Minute); err != nil || !ok {
			b.Fatalf("SetAccountVerifyToken = %v, %v", ok, err)
		}
		_ = r.client.Del(ctx, RedisKeyAccountToken(account)).Err()
	}
}

func BenchmarkIntegrationSetUserSessionRecord(b *testing.B) {
	r, ctx := newIntegrationRedisForBenchmark(b)
	uid := uint64(9000000000000000)
	records := map[string]string{
		userSessionFieldGatewayKey:  "gateway",
		userSessionFieldOnlineKey:   "online",
		userSessionFieldUserSession: "session",
	}

	for i := 0; i < b.N; i++ {
		if err := r.SetUserSessionRecord(ctx, uid+uint64(i), records); err != nil {
			b.Fatalf("SetUserSessionRecord failed: %v", err)
		}
		_ = r.client.Del(ctx, RedisKeyUserSession(uid+uint64(i))).Err()
	}
}

func newIntegrationRedisForBenchmark(b *testing.B) (*Redis, context.Context) {
	b.Helper()
	return newIntegrationRedis(b)
}
