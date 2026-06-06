package common

import (
	"encoding/hex"
	"testing"
)

func TestRandomSessionValuesAreHex32(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (string, error)
	}{
		{name: "heartbeat session", fn: NewHeartbeatSession},
		{name: "user session", fn: NewUserSession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.fn()
			if err != nil {
				t.Fatalf("generate value failed: %v", err)
			}
			if len(value) != 64 {
				t.Fatalf("hex length = %d, want 64", len(value))
			}
			data, err := hex.DecodeString(value)
			if err != nil {
				t.Fatalf("decode hex failed: %v", err)
			}
			if len(data) != 32 {
				t.Fatalf("data length = %d, want 32", len(data))
			}
		})
	}
}
