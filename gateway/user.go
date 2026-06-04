package main

import (
	"fmt"
	"time"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xheartbeat "github.com/75912001/xlib/heartbeat"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xtimer "github.com/75912001/xlib/timer"
	"google.golang.org/protobuf/proto"
)

// User 表示一个客户端连接的会话上下文。
type User struct {
	uid     uint64
	account string
	remote  xnetcommon.IRemote
	ip      string
	online  *Online
	actor   *xactor.Actor[string]

	gatewaySession string
	// 固定连接身份，一次登录生成，心跳不轮换。
	userSession string

	verifyTimer *xtimer.Second
	hb          xheartbeat.HeartBeat
}

// newUser 创建用户 actor，并启动未验证超时定时器。
func newUser(remote xnetcommon.IRemote) *User {
	u := &User{remote: remote, ip: remote.GetIP()}
	u.actor = xactor.NewActor[string](fmt.Sprintf("%p", remote), nil, u.behavior)
	u.actor.Start()
	u.startVerifyTimer()
	return u
}

func sendClientRes(remote xnetcommon.IRemote, messageID uint32, sessionID uint32, resultID uint32, key uint64, message proto.Message) error {
	if remote == nil || !remote.IsConnect() {
		return nil
	}
	return remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: messageID,
			SessionID: sessionID,
			ResultID:  resultID,
			Key:       key,
		},
		PBMessage: message,
	})
}

func (p *User) IsVerified() bool {
	return p.online != nil || p.uid != 0 || p.verifyTimer == nil
}

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

// startVerifyTimer 注册验证超时回调，未验证完成则断开连接。
func (p *User) startVerifyTimer() {
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if p.IsClosed() || p.online != nil {
			return nil
		}
		xlog.PrintInfo(fmt.Sprintf("user[%s] verify timeout, disconnect", p.ip))
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return nil
	})
	p.verifyTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(GCfgCustomVerifyExpireTimeDuration/time.Second), p.actor)
}

// OnVerified 在登录验证成功后绑定 uid、account、online 和 gatewaySession。
func (p *User) OnVerified(uid uint64, account string, online *Online, gatewaySession string, userSession string) error {
	if p.IsClosed() {
		return fmt.Errorf("remote disconnected")
	}
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if account == "" {
		return fmt.Errorf("account is empty")
	}
	if online == nil {
		return fmt.Errorf("online is nil")
	}
	if gatewaySession == "" {
		return fmt.Errorf("gatewaySession is empty")
	}
	if userSession == "" {
		return fmt.Errorf("userSession is empty")
	}
	p.uid = uid
	p.account = account
	p.online = online
	p.gatewaySession = gatewaySession
	p.userSession = userSession
	GUserMgr.BindUID(uid, p)
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.restartHeartbeatTimer()
	return nil
}

func (p *User) UpdateGatewaySession(newGatewaySession string) error {
	if p.IsClosed() {
		return fmt.Errorf("remote disconnected")
	}
	if p.uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if p.online == nil {
		return fmt.Errorf("online is nil")
	}
	if p.gatewaySession == "" {
		return fmt.Errorf("gatewaySession is empty")
	}
	if p.userSession == "" {
		return fmt.Errorf("userSession is empty")
	}
	if newGatewaySession == "" {
		return fmt.Errorf("new gatewaySession is empty")
	}
	if p.gatewaySession == newGatewaySession {
		return nil
	}
	oldGatewaySession := p.gatewaySession
	if err := unaryOnlineUserUpdateGatewaySession(p.online, p.uid, xetcd.GEtcd.GetKey(), oldGatewaySession, newGatewaySession, p.userSession); err != nil {
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return err
	}
	p.gatewaySession = newGatewaySession
	return nil
}

// restartHeartbeatTimer 启动或重置心跳超时定时器。
func (p *User) restartHeartbeatTimer() {
	cb := xcontrol.NewCallBack(
		func(args ...any) error {
			if p.IsClosed() {
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

// Cleanup 在连接断开后清理定时器，并通知 online 清理当前 gatewaySession。
func (p *User) Cleanup(reason xnetcommon.DisconnectReason) {
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.hb.Stop()

	uid := p.uid
	online := p.online
	gatewaySession := p.gatewaySession
	userSession := p.userSession
	p.online = nil
	p.gatewaySession = ""
	p.userSession = ""
	p.account = ""

	if uid != 0 && online != nil && gatewaySession != "" && userSession != "" {
		if err := unaryOnlineUserOffline(online, uid, xetcd.GEtcd.GetKey(), gatewaySession, userSession, reason, "gateway user offline"); err != nil {
			xlog.GLog.Warnf("notify offline failed uid:%d reason:%d online:%s err:%v", uid, reason, online.Key, err)
		}
	}
}
