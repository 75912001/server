package main

import (
	"fmt"
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xmap "github.com/75912001/xlib/map"
	xtimer "github.com/75912001/xlib/timer"
)

// GLoginSessionMgr 保存 login 服务预写入的待验证登录会话。
var GLoginSessionMgr = newLoginSessionMgr()

// loginSessionMgr 管理 gateway 本地 pending login session。
//
// pending 只存在于当前 gateway 进程内，用于衔接 login HTTP 返回和客户端 TCP UserVerifyReq。
type loginSessionMgr struct {
	m *xmap.MapMutexMgr[string, *pendingLoginSession] // key: loginSessionKey(uid, gatewaySession)
}

// pendingLoginSession 表示一次尚未被客户端 TCP 验证消费的登录准备记录。
type pendingLoginSession struct {
	uid            uint64 // login 已分配的用户 ID。
	account        string // 账号标识，验证完成后绑定到 User。
	gatewayNonce   string // login 生成的一次性随机串，用于校验 gatewaySession。
	gatewaySession string // login 和 gateway 按 uid/gatewayKey/gatewayNonce 共同计算出的登录凭证。
}

func newLoginSessionMgr() *loginSessionMgr {
	return &loginSessionMgr{
		m: xmap.NewMapMutexMgr[string, *pendingLoginSession](),
	}
}

// loginSessionKey 生成 pending login session 的本地索引。
//
// gatewaySession 每次登录准备都会重新生成，因此同一个 uid 下也可以同时存在多次未消费登录准备。
func loginSessionKey(uid uint64, gatewaySession string) string {
	return fmt.Sprintf("%d:%s", uid, gatewaySession)
}

// Add 写入一条 pending login session，并使用 xlib 全局秒级定时器注册过期删除。
//
// 定时器句柄不保存到 pending，Consume 不取消定时器；已消费的 pending 到期后再次 Expire 是 no-op。
func (p *loginSessionMgr) Add(uid uint64, account string, gatewayNonce string, gatewaySession string, ttl time.Duration) error {
	if xtimer.GTimer == nil {
		return fmt.Errorf("global timer is nil")
	}
	if GGatewayServer == nil || GGatewayServer.Server == nil || GGatewayServer.Server.GetActor() == nil {
		return fmt.Errorf("gateway server actor is nil")
	}

	key := loginSessionKey(uid, gatewaySession)
	pending := &pendingLoginSession{
		uid:            uid,
		account:        account,
		gatewayNonce:   gatewayNonce,
		gatewaySession: gatewaySession,
	}
	xtimer.GTimer.AddSecond(xcontrol.NewCallBack(
		func(args ...any) error {
			p.Expire(key)
			return nil
		},
	), time.Now().Unix()+int64(ttl/time.Second), GGatewayServer.Server.GetActor())

	p.m.Add(key, pending)
	return nil
}

// Consume 按 uid 和 gatewaySession 消费 pending。
//
// pending 只能被消费一次；Consume 只删除 map key，不触碰 xlib timer，避免 unary 调用路径和 timer 状态竞争。
func (p *loginSessionMgr) Consume(uid uint64, gatewaySession string) (*pendingLoginSession, bool) {
	key := loginSessionKey(uid, gatewaySession)
	pending, ok := p.m.Find(key)
	if ok {
		p.m.Del(key)
	} else {
		return nil, false
	}
	return pending, true
}

// Expire 删除到期仍未被消费的 pending。
//
// 这里由 xlib 全局定时器回调触发；key 已在 Add 时固定，过期后直接从本地 map 删除。
// 如果 pending 已被 Consume 删除，则本次删除是 no-op。
func (p *loginSessionMgr) Expire(key string) {
	p.m.Del(key)
}
