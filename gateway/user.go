package main

import (
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xheartbeat "github.com/75912001/xlib/heartbeat"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xtimer "github.com/75912001/xlib/timer"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// User 一个客户端连接的会话上下文。
//   - online：校验成功后绑定的 online 实例，用于后续业务包透传（stream.Send）。
//   - verified：校验是否通过，原子标志，定时器超时回调用它快速判断。
//   - verifyTimer / hb：校验超时定时器 + 心跳管理（二选一存在）。
//   - hb.WaitID：上次下发给客户端的 session（防重放），首次心跳时为 0。
type User struct {
	uid     atomic.Uint64      // 玩家唯一 ID，校验成功后填充
	remote  xnetcommon.IRemote // 客户端 TCP 连接（发包 / 主动断开）
	ip      string
	online  *Online     // 校验后绑定的 online 实例（用 selector 的同一份哈希）
	verifyd atomic.Bool // 校验是否通过
	closed  atomic.Bool
	shard   *UserShard

	verifyTimer *xtimer.Second       // 校验超时定时器（onVerified 后置 nil）
	hb          xheartbeat.HeartBeat // 心跳管理（WaitID=lastSession，Stop/Start 封装定时器）
}

// newUser 创建用户并启动「未校验超时」定时器（必须在 UserShard actor 初始化之后调用）。
func newUser(remote xnetcommon.IRemote, shard *UserShard) *User {
	u := &User{remote: remote, ip: remote.GetIP(), shard: shard}
	u.startVerifyTimer()
	return u
}

// IsVerified 校验是否通过（外部安全读取）
func (u *User) IsVerified() bool { return u.verifyd.Load() }

// GetOnline 返回校验后绑定的 online 实例（未校验时为 nil）
func (u *User) GetOnline() *Online { return u.online }

func (u *User) Disconnect(reason xnetcommon.DisconnectReason) {
	if !u.closed.CompareAndSwap(false, true) {
		return
	}
	u.remote.SetDisconnectReason(reason)
	u.remote.Stop()
}

// startVerifyTimer 注册超时回调：到期若仍未校验则断开连接
func (u *User) startVerifyTimer() {
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if u.closed.Load() || u.verifyd.Load() {
			return nil
		}
		xlog.PrintInfo(fmt.Sprintf("user[%s] verify timeout, disconnect", u.ip))
		u.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return nil
	})
	u.verifyTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+cfgVerifyExpireTimeSecond(), u.shard.actor)
}

// OnVerified 由登录鉴权成功后调用：绑定 uid + online，停校验定时器，启心跳定时器。
func (u *User) OnVerified(uid uint64, online *Online) {
	if u.closed.Load() {
		return
	}
	u.uid.Store(uid)
	u.online = online
	u.verifyd.Store(true)
	GUserMgr.BindUID(uid, u)
	if u.verifyTimer != nil {
		xtimer.GTimer.DelSecond(u.verifyTimer)
		u.verifyTimer = nil
	}
	u.startHeartbeatTimer()
}

// startHeartbeatTimer 启动 / 重置心跳超时定时器（由 UserShard actor 串行调用）
func (u *User) startHeartbeatTimer() {
	cb := xcontrol.NewCallBack(
		func(args ...any) error {
			if u.closed.Load() {
				return nil
			}
			xlog.PrintInfo(fmt.Sprintf("user[uid=%d] heartbeat timeout, disconnect", u.uid.Load()))
			u.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
			return nil
		},
		u, xtimer.GTimer, cfgHeartBeatExpireSecond())
	u.hb.Stop()
	u.hb.Start(cb, u.shard.actor)
}

// OnHeartbeatReq 处理客户端心跳请求。
//
//	验证 last_session 与上一次下发的 session 是否一致（首次允许 0）；
//	若不一致视为重放/篡改，主动断开；
//	否则生成新 session 并下发，重置心跳超时定时器。
func (u *User) OnHeartbeatReq(header *xpacket.Header, body []byte) error {
	if u.closed.Load() {
		return nil
	}
	if !u.verifyd.Load() {
		xlog.PrintfErr("heartbeat before verify, remote=%s", u.ip)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}

	var req pb.UserHeartbeatReq
	if err := proto.Unmarshal(body, &req); err != nil {
		return errors.WithMessage(err, "UserHeartbeatReq unmarshal")
	}

	if u.hb.WaitID != 0 && req.GetLastSession() != u.hb.WaitID {
		xlog.PrintfErr("user[uid=%d] heartbeat session mismatch: got=%d expect=%d",
			u.uid.Load(), req.GetLastSession(), u.hb.WaitID)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return errors.New("heartbeat session mismatch")
	}

	next := genSession(u.hb.WaitID)
	u.hb.WaitID = next

	u.startHeartbeatTimer()

	return u.remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDUser_UserHeartbeatRes_CMD),
			SessionID: header.SessionID,
			Key:       header.Key,
		},
		PBMessage: &pb.UserHeartbeatRes{
			ServerTime:  uint64(time.Now().UnixMilli()),
			NextSession: next,
		},
	})
}

// Cleanup 连接断开后清理所有定时器（由 OnDisconnect 调用）。
func (u *User) Cleanup() {
	if u.verifyTimer != nil {
		xtimer.GTimer.DelSecond(u.verifyTimer)
		u.verifyTimer = nil
	}
	u.hb.Stop()
	u.online = nil
}

// genSession 产生一个非零、且不等于 prev 的随机 session
func genSession(prev uint32) uint32 {
	for {
		n := rand.Uint32()
		if n != 0 && n != prev {
			return n
		}
	}
}
