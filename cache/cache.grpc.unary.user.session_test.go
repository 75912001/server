package main

import (
	"reflect"
	"testing"
)

func TestCacheUserSessionRecordMapRequiresMetadata(t *testing.T) {
	if _, ok := cacheUserSessionRecordMap("gateway-1", "session-1", 0, "online-1"); ok {
		t.Fatal("cacheUserSessionRecordMap accepted missing loginTime")
	}
	if _, ok := cacheUserSessionRecordMap("gateway-1", "session-1", 123, ""); ok {
		t.Fatal("cacheUserSessionRecordMap accepted missing onlineKey")
	}

	got, ok := cacheUserSessionRecordMap("gateway-1", "session-1", 123, "online-1")
	if !ok {
		t.Fatal("cacheUserSessionRecordMap returned false")
	}
	want := map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldUserSession: "session-1",
		userSessionFieldLoginTime:   "123",
		userSessionFieldOnlineKey:   "online-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestCacheUserSessionFromMap(t *testing.T) {
	if _, ok := cacheUserSessionFromMap(map[string]string{}); ok {
		t.Fatal("cacheUserSessionFromMap accepted empty map")
	}

	got, ok := cacheUserSessionFromMap(map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldUserSession: "session-1",
		userSessionFieldLoginTime:   "123",
		userSessionFieldOnlineKey:   "online-1",
	})
	if !ok {
		t.Fatal("cacheUserSessionFromMap returned false")
	}
	if got.GetGatewayKey() != "gateway-1" || got.GetUserSession() != "session-1" || got.GetLoginTimeMs() != 123 || got.GetOnlineKey() != "online-1" {
		t.Fatalf("session = %#v", got)
	}
}
