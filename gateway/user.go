package main

import (
	"fmt"
	"server/common"
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

// User 表示一个客户端连接的会话上下文。
type User struct {
	uid     uint64
	account string
	remote  xnetcommon.IRemote
	ip      string
	online  *Online
	actor   *xactor.Actor[string]

	userSession string // 固定连接身份，一次登录生成，心跳不轮换。

	verifyTimer *xtimer.Second

	heartbeatTimer   *xtimer.Second
	heartbeatSession string
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
	if _, err := GUserMgr.Remove(p.remote); err != nil {
		xlog.GLog.Warnf("phase=disconnect_cleanup uid=%d reason=%d err=%v", p.uid, reason, err)
	}
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

// OnVerified 在登录验证成功后绑定 uid、account、online 和 heartbeatSession。
func (p *User) OnVerified(uid uint64, account string, online *Online, heartbeatSession string, userSession string) error {
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
	if heartbeatSession == "" {
		return fmt.Errorf("heartbeatSession is empty")
	}
	if userSession == "" {
		return fmt.Errorf("userSession is empty")
	}
	p.uid = uid
	p.account = account
	p.online = online
	p.heartbeatSession = heartbeatSession
	p.userSession = userSession
	GUserMgr.BindUID(uid, p)
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	p.restartHeartbeatTimer()
	return nil
}

func (p *User) UpdateHeartbeatSession(newHeartbeatSession string) error {
	if p.IsClosed() {
		return fmt.Errorf("remote disconnected")
	}
	if p.uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if p.online == nil {
		return fmt.Errorf("online is nil")
	}
	if p.heartbeatSession == "" {
		return fmt.Errorf("heartbeatSession is empty")
	}
	if p.userSession == "" {
		return fmt.Errorf("userSession is empty")
	}
	if newHeartbeatSession == "" {
		return fmt.Errorf("new heartbeatSession is empty")
	}
	if p.heartbeatSession == newHeartbeatSession {
		return nil
	}
	if err := unaryCacheRefreshUserSession(p.uid, p.userSession); err != nil {
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return err
	}
	p.heartbeatSession = newHeartbeatSession
	return nil
}

// restartHeartbeatTimer 启动或重置心跳超时定时器。
func (p *User) restartHeartbeatTimer() {
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelSecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}
	cb := xcontrol.NewCallBack(
		func(args ...any) error {
			if p.IsClosed() {
				return nil
			}
			xlog.PrintInfo(fmt.Sprintf("user[uid=%d] heartbeat timeout, disconnect", p.uid))
			p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
			return nil
		})
	p.heartbeatTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(GCfgCustomHeartBeatExpireDuration/time.Second), p.actor)
}

// Cleanup 在连接断开后清理定时器，并通知 online/cache 清理当前 userSession。
func (p *User) Cleanup(reason xnetcommon.DisconnectReason) error {
	if p.verifyTimer != nil {
		xtimer.GTimer.DelSecond(p.verifyTimer)
		p.verifyTimer = nil
	}
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelSecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}

	uid := p.uid
	online := p.online
	userSession := p.userSession
	p.online = nil
	p.heartbeatSession = ""
	p.userSession = ""
	p.account = ""

	var err error
	if uid != 0 && userSession != "" {
		gatewayKey := xetcd.GEtcd.GetKey()
		if online != nil {
			if errTmp := unaryOnlineUnbindUser(online, uid, gatewayKey, userSession, reason, "gateway user offline"); errTmp != nil {
				err = xerror.AppendError(err, errTmp)
				xlog.GLog.Warnf("phase=cleanup_online uid=%d gatewayKey=%s onlineKey=%s userSession=%s reason=%d err=%v",
					uid, gatewayKey, online.Key, common.ShortSession(userSession), reason, errTmp)
			}
		}
		if errTmp := unaryCacheEndUserSession(uid, userSession); errTmp != nil {
			err = xerror.AppendError(err, errTmp)
			xlog.GLog.Warnf("phase=cleanup_cache uid=%d gatewayKey=%s userSession=%s reason=%d err=%v",
				uid, gatewayKey, common.ShortSession(userSession), reason, errTmp)
		}
	}
	return err
}
