package main

import (
	"fmt"
	"time"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xtimer "github.com/75912001/xlib/timer"
	"google.golang.org/protobuf/proto"
)

// Account 表示一个客户端连接的会话上下文。
type Account struct {
	aid     uint64
	account string
	remote  xnetcommon.IRemote
	ip      string
	online  *Online
	actor   *xactor.Actor[string]

	accountSession string // 固定连接身份，一次登录生成，心跳不轮换。

	verifyTimer *xtimer.Second

	heartbeatTimer   *xtimer.Second
	heartbeatSession string
}

// newAccount 创建用户 actor，并启动未验证超时定时器。
func newAccount(remote xnetcommon.IRemote) *Account {
	u := &Account{remote: remote, ip: remote.GetIP()}
	u.actor = xactor.NewActor[string](fmt.Sprintf("%p", remote), nil, u.behavior)
	u.actor.Start()
	u.startVerifyTimer()
	return u
}

func sendClientRes(remote xnetcommon.IRemote, messageID uint32, resultID uint32, key uint64, message proto.Message) error {
	if remote == nil || !remote.IsConnect() {
		return nil
	}
	return remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: messageID,
			SessionID: 0,
			ResultID:  resultID,
			Key:       key,
		},
		PBMessage: message,
	})
}

func (p *Account) IsVerified() bool {
	return p.online != nil || p.aid != 0 || p.verifyTimer == nil
}

func (p *Account) IsClosed() bool {
	return !p.remote.IsConnect()
}

func (p *Account) Disconnect(reason xnetcommon.DisconnectReason) {
	if p.IsClosed() {
		return
	}
	p.remote.SetDisconnectReason(reason)
	if _, err := GAccountMgr.Remove(p.remote); err != nil {
		xlog.GLog.Warnf("phase=disconnect_cleanup aid=%d reason=%d err=%v", p.aid, reason, err)
	}
}

// startVerifyTimer 注册验证超时回调，未验证完成则断开连接。
func (p *Account) startVerifyTimer() {
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if p.IsClosed() || p.online != nil {
			return nil
		}
		xlog.PrintInfo(fmt.Sprintf("account[%s] verify timeout, disconnect", p.ip))
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return nil
	})
	p.verifyTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(GCfgCustomVerifyExpireTimeDuration/time.Second), p.actor)
}

// OnVerified 在登录验证成功后绑定 aid、account、online 和 heartbeatSession。
func (p *Account) OnVerified(aid uint64, account string, online *Online, heartbeatSession string, accountSession string) error {
	if p.IsClosed() {
		return fmt.Errorf("remote disconnected")
	}
	if aid == 0 {
		return fmt.Errorf("aid is empty")
	}
	if account == "" {
		return fmt.Errorf("account is empty")
	}
	if online == nil {
		return fmt.Errorf("online is nil")
	}
	if heartbeatSession == "" {
		return fmt.Errorf("heartbeatSession is empty")
	}
	if accountSession == "" {
		return fmt.Errorf("accountSession is empty")
	}
	p.aid = aid
	p.account = account
	p.online = online
	p.heartbeatSession = heartbeatSession
	p.accountSession = accountSession
	GAccountMgr.BindAID(aid, p)
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.restartHeartbeatTimer()
	return nil
}

func (p *Account) UpdateHeartbeatSession(newHeartbeatSession string) error {
	if p.IsClosed() {
		return fmt.Errorf("remote disconnected")
	}
	if p.aid == 0 {
		return fmt.Errorf("aid is empty")
	}
	if p.online == nil {
		return fmt.Errorf("online is nil")
	}
	if p.heartbeatSession == "" {
		return fmt.Errorf("heartbeatSession is empty")
	}
	if p.accountSession == "" {
		return fmt.Errorf("accountSession is empty")
	}
	if newHeartbeatSession == "" {
		return fmt.Errorf("new heartbeatSession is empty")
	}
	if p.heartbeatSession == newHeartbeatSession {
		return nil
	}
	if err := unaryCacheRefreshAccountSession(p.aid, p.accountSession); err != nil {
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return err
	}
	p.heartbeatSession = newHeartbeatSession
	return nil
}

// restartHeartbeatTimer 启动或重置心跳超时定时器。
func (p *Account) restartHeartbeatTimer() {
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelSecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}
	cb := xcontrol.NewCallBack(
		func(args ...any) error {
			if p.IsClosed() {
				return nil
			}
			xlog.PrintInfo(fmt.Sprintf("account[aid=%d] heartbeat timeout, disconnect", p.aid))
			p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
			return nil
		})
	p.heartbeatTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(GCfgCustomHeartBeatExpireDuration/time.Second), p.actor)
}

// Cleanup 在连接断开后清理定时器，并通知 online/cache 清理当前 accountSession。
func (p *Account) Cleanup(reason xnetcommon.DisconnectReason) error {
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelSecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}

	aid := p.aid
	online := p.online
	accountSession := p.accountSession
	p.online = nil
	p.heartbeatSession = ""
	p.accountSession = ""
	p.account = ""

	var err error
	if aid != 0 && accountSession != "" {
		gatewayKey := xetcd.GEtcd.GetKey()
		if online != nil {
			if errTmp := unaryOnlineUnbindAccount(online, aid, gatewayKey, accountSession, reason, "gateway account offline"); errTmp != nil {
				err = xerror.AppendError(err, errTmp)
				xlog.GLog.Warnf("phase=cleanup_online aid=%d gatewayKey=%s onlineKey=%s accountSession=%s reason=%d err=%v",
					aid, gatewayKey, online.Key, accountSession, reason, errTmp)
			}
		}
		if errTmp := unaryCacheEndAccountSession(aid, accountSession); errTmp != nil {
			err = xerror.AppendError(err, errTmp)
			xlog.GLog.Warnf("phase=cleanup_cache aid=%d gatewayKey=%s accountSession=%s reason=%d err=%v",
				aid, gatewayKey, accountSession, reason, errTmp)
		}
	}
	return err
}
