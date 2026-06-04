package main

import "testing"

func TestRedisKeyFunctions(t *testing.T) {
	setTestCacheConfig(t)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "user record", got: RedisKeyUserRecord(1001), want: "user:{1001}:record"},
		{name: "user session", got: RedisKeyUserSession(1001), want: "user:{1001}:session"},
		{name: "account token", got: RedisKeyAccountToken("alice"), want: "account:{alice}:token"},
		{name: "account uid", got: RedisKeyAccountUID("alice"), want: "account:{alice}:uid"},
		{name: "account lock", got: RedisKeyAccountLock("alice"), want: "account:{alice}:lock"},
		{name: "group uid sequence", got: RedisKeyUserUIDSequence(2), want: "user:uid:sequence:{2}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("key = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
