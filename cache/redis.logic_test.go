package main

import "testing"

func TestRedisScriptResultIsOK(t *testing.T) {
	tests := []struct {
		name   string
		result any
		want   bool
	}{
		{name: "int64 ok", result: int64(1), want: true},
		{name: "int64 false", result: int64(0), want: false},
		{name: "int ok", result: int(1), want: true},
		{name: "string ok", result: "1", want: true},
		{name: "string false", result: "0", want: false},
		{name: "unsupported", result: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redisScriptResultIsOK(tt.result); got != tt.want {
				t.Fatalf("redisScriptResultIsOK(%#v) = %v, want %v", tt.result, got, tt.want)
			}
		})
	}
}
