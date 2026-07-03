package main

import (
	"reflect"
	"testing"
)

func TestCacheAccountSessionRecordMapRequiresMetadata(t *testing.T) {
	if _, ok := cacheAccountSessionRecordMap("gateway-1", "session-1", 0, "online-1"); ok {
		t.Fatal("cacheAccountSessionRecordMap accepted missing loginTimestampMs")
	}
	if _, ok := cacheAccountSessionRecordMap("gateway-1", "session-1", 123, ""); ok {
		t.Fatal("cacheAccountSessionRecordMap accepted missing onlineKey")
	}

	got, ok := cacheAccountSessionRecordMap("gateway-1", "session-1", 123, "online-1")
	if !ok {
		t.Fatal("cacheAccountSessionRecordMap returned false")
	}
	want := map[string]string{
		accountSessionFieldGatewayKey:       "gateway-1",
		accountSessionFieldAccountSession:   "session-1",
		accountSessionFieldLoginTimestampMs: "123",
		accountSessionFieldOnlineKey:        "online-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestCacheAccountSessionFromMap(t *testing.T) {
	if _, ok := cacheAccountSessionFromMap(map[string]string{}); ok {
		t.Fatal("cacheAccountSessionFromMap accepted empty map")
	}

	got, ok := cacheAccountSessionFromMap(map[string]string{
		accountSessionFieldGatewayKey:       "gateway-1",
		accountSessionFieldAccountSession:   "session-1",
		accountSessionFieldLoginTimestampMs: "123",
		accountSessionFieldOnlineKey:        "online-1",
	})
	if !ok {
		t.Fatal("cacheAccountSessionFromMap returned false")
	}
	if got.GetGatewayKey() != "gateway-1" || got.GetAccountSession() != "session-1" || got.GetLoginTimestampMs() != 123 || got.GetOnlineKey() != "online-1" {
		t.Fatalf("session = %#v", got)
	}
}
