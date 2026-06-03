package main

import (
	"fmt"
	"sync"
	"time"
)

var GLoginSessionMgr = newLoginSessionMgr()

type loginSessionMgr struct {
	mu sync.Mutex
	m  map[string]*pendingLoginSession
}

type pendingLoginSession struct {
	uid            uint64
	account        string
	gatewayNonce   string
	gatewaySession string
	timer          *time.Timer
}

func newLoginSessionMgr() *loginSessionMgr {
	return &loginSessionMgr{
		m: make(map[string]*pendingLoginSession),
	}
}

func loginSessionKey(uid uint64, gatewaySession string) string {
	return fmt.Sprintf("%d:%s", uid, gatewaySession)
}

func (p *loginSessionMgr) Add(uid uint64, account string, gatewayNonce string, gatewaySession string, ttl time.Duration) {
	key := loginSessionKey(uid, gatewaySession)
	pending := &pendingLoginSession{
		uid:            uid,
		account:        account,
		gatewayNonce:   gatewayNonce,
		gatewaySession: gatewaySession,
	}
	pending.timer = time.AfterFunc(ttl, func() {
		p.Expire(uid, gatewaySession, pending)
	})

	p.mu.Lock()
	old := p.m[key]
	p.m[key] = pending
	p.mu.Unlock()

	if old != nil && old.timer != nil {
		old.timer.Stop()
	}
}

func (p *loginSessionMgr) Consume(uid uint64, gatewaySession string) (*pendingLoginSession, bool) {
	key := loginSessionKey(uid, gatewaySession)
	p.mu.Lock()
	pending, ok := p.m[key]
	if ok {
		delete(p.m, key)
	}
	p.mu.Unlock()
	if !ok {
		return nil, false
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	return pending, true
}

func (p *loginSessionMgr) Expire(uid uint64, gatewaySession string, pending *pendingLoginSession) {
	key := loginSessionKey(uid, gatewaySession)
	p.mu.Lock()
	if p.m[key] == pending {
		delete(p.m, key)
	}
	p.mu.Unlock()
}
