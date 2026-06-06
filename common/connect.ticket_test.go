package common

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConnectTicketSignVerify(t *testing.T) {
	now := time.UnixMilli(1717599970000)
	payload, err := NewConnectTicketPayload(10001, "robot.10001", "gateway-1", 30*time.Second, now)
	if err != nil {
		t.Fatalf("new payload failed: %v", err)
	}
	ticket, err := SignConnectTicket(payload, "secret")
	if err != nil {
		t.Fatalf("sign ticket failed: %v", err)
	}
	got, err := VerifyConnectTicket(ticket, ConnectTicketVerifyOptions{
		Secret:     "secret",
		GatewayKey: "gateway-1",
		UID:        10001,
		Now:        now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("verify ticket failed: %v", err)
	}
	if got.Account != "robot.10001" {
		t.Fatalf("payload = %+v", got)
	}

	parts := strings.Split(ticket, ".")
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode payload failed: %v", err)
	}
	var fields map[string]any
	if err = json.Unmarshal(payloadJSON, &fields); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if _, ok := fields["heartbeatSession"]; ok {
		t.Fatalf("connect ticket payload should not contain heartbeatSession: %s", payloadJSON)
	}
	if _, ok := fields["keyId"]; ok {
		t.Fatalf("connect ticket payload should not contain keyId: %s", payloadJSON)
	}
}

func TestConnectTicketVerifyRejectsInvalidTicket(t *testing.T) {
	now := time.UnixMilli(1717599970000)
	payload, err := NewConnectTicketPayload(10001, "robot.10001", "gateway-1", 30*time.Second, now)
	if err != nil {
		t.Fatalf("new payload failed: %v", err)
	}
	ticket, err := SignConnectTicket(payload, "secret")
	if err != nil {
		t.Fatalf("sign ticket failed: %v", err)
	}
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 {
		t.Fatalf("unexpected ticket format: %s", ticket)
	}

	tests := []struct {
		name string
		tick string
		opts ConnectTicketVerifyOptions
		want error
	}{
		{
			name: "tampered payload",
			tick: parts[0] + "x." + parts[1],
			opts: ConnectTicketVerifyOptions{Secret: "secret", Now: now},
			want: ErrConnectTicketSignMismatch,
		},
		{
			name: "expired",
			tick: ticket,
			opts: ConnectTicketVerifyOptions{Secret: "secret", Now: now.Add(31 * time.Second)},
			want: ErrConnectTicketExpired,
		},
		{
			name: "wrong gateway",
			tick: ticket,
			opts: ConnectTicketVerifyOptions{Secret: "secret", GatewayKey: "gateway-2", Now: now},
			want: ErrConnectTicketKeyMismatch,
		},
		{
			name: "wrong uid",
			tick: ticket,
			opts: ConnectTicketVerifyOptions{Secret: "secret", UID: 10002, Now: now},
			want: ErrConnectTicketUIDMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyConnectTicket(tt.tick, tt.opts)
			if err != tt.want {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
