package main

import (
	"strconv"
	"testing"
)

func BenchmarkCacheUserSessionRecordMap(b *testing.B) {
	for b.Loop() {
		cacheUserSessionRecordMap("gateway-1", "session-1", 123456, "online-1")
	}
}

func BenchmarkCacheUserSessionFromMap(b *testing.B) {
	records := map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldUserSession: "session-1",
		userSessionFieldLoginTime:   strconv.FormatInt(123456, 10),
		userSessionFieldOnlineKey:   "online-1",
	}

	for b.Loop() {
		cacheUserSessionFromMap(records)
	}
}
