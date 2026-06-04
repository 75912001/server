package main

import "testing"

func TestLoginSessionMgrConsumeOnce(t *testing.T) {
	mgr := newLoginSessionMgr()
	mgr.m.Add(loginSessionKey(1001, "gateway-session-1"), &pendingLoginSession{
		uid:            1001,
		account:        "account-1",
		gatewayNonce:   "nonce-1",
		gatewaySession: "gateway-session-1",
	})

	pending, ok := mgr.Consume(1001, "gateway-session-1")
	if !ok {
		t.Fatalf("expected pending login session")
	}
	if pending.uid != 1001 || pending.account != "account-1" || pending.gatewayNonce != "nonce-1" {
		t.Fatalf("unexpected pending login session: %+v", pending)
	}

	if pending, ok := mgr.Consume(1001, "gateway-session-1"); ok || pending != nil {
		t.Fatalf("pending login session should be consumed once, got pending=%+v ok=%v", pending, ok)
	}
}

func TestLoginSessionMgrExpireDeletesPending(t *testing.T) {
	mgr := newLoginSessionMgr()
	key := loginSessionKey(1001, "gateway-session-1")

	mgr.m.Add(key, &pendingLoginSession{
		uid:            1001,
		account:        "account-1",
		gatewayNonce:   "nonce-1",
		gatewaySession: "gateway-session-1",
	})
	mgr.Expire(key)

	if pending, ok := mgr.Consume(1001, "gateway-session-1"); ok || pending != nil {
		t.Fatalf("pending login session should be expired, got pending=%+v ok=%v", pending, ok)
	}
}

func TestLoginSessionMgrExpireAfterConsumeIsNoop(t *testing.T) {
	mgr := newLoginSessionMgr()
	key := loginSessionKey(1001, "gateway-session-1")

	mgr.m.Add(key, &pendingLoginSession{
		uid:            1001,
		account:        "account-1",
		gatewayNonce:   "nonce-1",
		gatewaySession: "gateway-session-1",
	})

	if pending, ok := mgr.Consume(1001, "gateway-session-1"); !ok || pending == nil {
		t.Fatalf("expected pending login session before expire")
	}

	mgr.Expire(key)

	if pending, ok := mgr.Consume(1001, "gateway-session-1"); ok || pending != nil {
		t.Fatalf("pending login session should stay removed after late expire, got pending=%+v ok=%v", pending, ok)
	}
}

func TestLoginSessionMgrAddRequiresTimerOut(t *testing.T) {
	mgr := newLoginSessionMgr()
	old := GGatewayServer
	GGatewayServer = nil
	t.Cleanup(func() {
		GGatewayServer = old
	})

	if err := mgr.Add(1001, "account-1", "nonce-1", "gateway-session-1", 1); err == nil {
		t.Fatalf("expected error when timer out is nil")
	}
}
