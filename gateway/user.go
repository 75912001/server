package main

import (
	"fmt"
	"time"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xheartbeat "github.com/75912001/xlib/heartbeat"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xtimer "github.com/75912001/xlib/timer"
)

// User 一个客户端连接的会话上下文。
//   - online：nil 表示未校验或已清理，非 nil 表示校验通过并绑定的 online 实例。
//   - verifyTimer / hb：校验超时定时器 + 心跳管理（二选一存在）。
//   - hb.WaitID：上次下发给客户端的 session（防重放），首次心跳时为 0。
type User struct {
	uid    uint64             // 玩家唯一 ID，校验成功后填充
	remote xnetcommon.IRemote // 客户端 TCP 连接（发包 / 主动断开）
	ip     string
	online *Online // nil 表示未校验或已清理，非 nil 表示校验通过并绑定的 online 实例
	actor  *xactor.Actor[string]

	verifyTimer *xtimer.Second       // 校验超时定时器（onVerified 后置 nil）
	hb          xheartbeat.HeartBeat // 心跳管理（WaitID=lastSession，Stop/Start 封装定时器）
}

// newUser 创建用户 actor，并启动「未校验超时」定时器。
func newUser(remote xnetcommon.IRemote) *User {
	u := &User{remote: remote, ip: remote.GetIP()}
	u.actor = xactor.NewActor[string](fmt.Sprintf("%p", remote), nil, u.behavior)
	u.actor.Start()
	u.startVerifyTimer()
	return u
}

// 是否校验成功
func (p *User) IsVerified() bool {
	return p.online != nil || p.uid != 0 || p.verifyTimer == nil
}

// 是否连接已断开
func (p *User) IsClosed() bool {
	return !p.remote.IsConnect()
}

func (p *User) Disconnect(reason xnetcommon.DisconnectReason) {
	if p.IsClosed() {
		return
	}
	p.remote.SetDisconnectReason(reason)
	GUserMgr.Remove(p.remote)
}

// startVerifyTimer 注册超时回调：到期若仍未校验则断开连接
func (p *User) startVerifyTimer() {
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if !p.remote.IsConnect() || p.online != nil {
			return nil
		}
		xlog.PrintInfo(fmt.Sprintf("user[%s] verify timeout, disconnect", p.ip))
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return nil
	})
	p.verifyTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(GCfgCustomverifyExpireTimeDuration/time.Second), p.actor)
}

// OnVerified 由登录鉴权成功后调用：绑定 uid + online，停校验定时器，启心跳定时器。
func (p *User) OnVerified(uid uint64, online *Online) {
	if !p.remote.IsConnect() {
		return
	}
	p.uid = uid
	p.online = online
	GUserMgr.BindUID(uid, p)
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.startHeartbeatTimer()
}

// startHeartbeatTimer 启动 / 重置心跳超时定时器（由用户 actor 串行调用）
func (p *User) startHeartbeatTimer() {
	cb := xcontrol.NewCallBack(

		func(args ...any) error {
			if !p.remote.IsConnect() {
				return nil
			}
			xlog.PrintInfo(fmt.Sprintf("user[uid=%d] heartbeat timeout, disconnect", p.uid))
			p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
			return nil
		},
		p, xtimer.GTimer, int64(GCfgCustomHeartBeatExpireDuration/time.Second))
	p.hb.Stop()
	p.hb.Start(cb, p.actor)
}

// Cleanup 连接断开后由 user actor 串行清理定时器和在线状态。
func (p *User) Cleanup(reason xnetcommon.DisconnectReason) {
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.hb.Stop()

	uid := p.uid

	online := p.online
	p.online = nil

	if uid != 0 && online != nil {
		if err := unaryOnlineUserOffline(online, uid, reason, "gateway user offline"); err != nil {
			xlog.GLog.Warnf("notify offline failed uid=%d reason=%d online=%s err=%v", uid, reason, online.ID, err)
		}
	}
}
