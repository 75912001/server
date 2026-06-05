package main

import (
	"testing"
	"time"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

type testRemote struct {
	ip        string
	connected bool
	reason    xnetcommon.DisconnectReason
}

func (p *testRemote) Send(xpacket.IPacket) error {
	return nil
}

func (p *testRemote) IsOverload(uint32, time.Time) bool {
	return false
}

func (p *testRemote) IsConnect() bool {
	return p.connected
}

func (p *testRemote) Start(*xnetcommon.ConnOptions, xcontrol.IOut, xnetcommon.IHandler) {
	p.connected = true
}

func (p *testRemote) Stop() {
	p.connected = false
}

func (p *testRemote) GetIP() string {
	return p.ip
}

func (p *testRemote) GetDisconnectReason() xnetcommon.DisconnectReason {
	return p.reason
}

func (p *testRemote) SetDisconnectReason(reason xnetcommon.DisconnectReason) {
	p.reason = reason
}

func newTestUser(uid uint64, remote xnetcommon.IRemote) *User {
	u := &User{uid: uid, remote: remote}
	u.actor = xactor.NewActor[string]("test-user", nil, u.behavior)
	u.actor.Start()
	return u
}

func TestUserMgrRemoveKeepsNewUIDBinding(t *testing.T) {
	resetTestGatewayUserMgr(t)

	const uid uint64 = 1001
	oldRemote := &testRemote{ip: "127.0.0.1", connected: true}
	newRemote := &testRemote{ip: "127.0.0.2", connected: true}
	oldUser := newTestUser(uid, oldRemote)
	newUser := &User{uid: uid, remote: newRemote}

	GUserMgr.byRemote.Add(oldRemote, oldUser)
	GUserMgr.byUID.Add(uid, newUser)

	removed, err := GUserMgr.Remove(oldRemote)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if removed != oldUser {
		t.Fatalf("removed = %#v, want old user", removed)
	}
	if got := GUserMgr.GetByUID(uid); got != newUser {
		t.Fatalf("uid binding = %#v, want new user %#v", got, newUser)
	}
}

func TestUserMgrRemoveDeletesMatchingUIDBinding(t *testing.T) {
	resetTestGatewayUserMgr(t)

	const uid uint64 = 1001
	remote := &testRemote{ip: "127.0.0.1", connected: true}
	user := newTestUser(uid, remote)

	GUserMgr.byRemote.Add(remote, user)
	GUserMgr.byUID.Add(uid, user)

	if _, err := GUserMgr.Remove(remote); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if got := GUserMgr.GetByUID(uid); got != nil {
		t.Fatalf("uid binding = %#v, want nil", got)
	}
}
