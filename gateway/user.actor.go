package main

import (
	"fmt"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

const (
	// CmdUserOnlineFrame 参数：*pb.OnlineTunnelFrame；online 下行给指定用户的业务包。
	CmdUserOnlineFrame xactor.CMD = 100
	// CmdUserVerified 参数：*userVerifiedEvent；登录成功后绑定 uid、online，并启动心跳定时器。
	CmdUserVerified xactor.CMD = 101
	// CmdUserHeartbeat 参数：*userHeartbeatEvent；处理客户端心跳请求并刷新心跳定时器。
	CmdUserHeartbeat xactor.CMD = 102
	// CmdUserCleanup 参数：*User；连接断开后清理用户定时器和状态。
	CmdUserCleanup xactor.CMD = 103
)

type userVerifiedEvent struct {
	user   *User
	uid    uint64
	online *Online
}

type userHeartbeatEvent struct {
	user   *User
	header *xpacket.Header
	body   []byte
}

func (s *UserShard) behavior(messages ...any) (xactor.Behavior, any, error) {
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				_ = event.ICallBack.Execute()
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case CmdUserOnlineFrame:
			frame, ok := msg.Args[0].(*pb.OnlineTunnelFrame)
			if ok {
				s.handleOnlineFrame(frame)
			}
		case CmdUserVerified:
			event, ok := msg.Args[0].(*userVerifiedEvent)
			if ok {
				event.user.OnVerified(event.uid, event.online)
			}
		case CmdUserHeartbeat:
			event, ok := msg.Args[0].(*userHeartbeatEvent)
			if ok {
				_ = event.user.OnHeartbeatReq(event.header, event.body)
			}
		case CmdUserCleanup:
			user, ok := msg.Args[0].(*User)
			if ok {
				user.Cleanup()
			}
		}
	}
	return s.behavior, nil, nil
}

func (s *UserShard) handleOnlineFrame(frame *pb.OnlineTunnelFrame) {
	uid := frame.GetUid()
	u := GUserMgr.GetByUID(uid)
	if u == nil {
		xlog.PrintfErr("user shard[%d]: uid=%d not found", s.id, uid)
		return
	}
	if u.closed.Load() {
		return
	}

	switch payload := frame.Payload.(type) {
	case *pb.OnlineTunnelFrame_KickUserReq:
		xlog.PrintInfo(fmt.Sprintf("kick uid=%d reason=%d msg=%s",
			uid, payload.KickUserReq.GetReason(), payload.KickUserReq.GetMsg()))
		u.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
	case *pb.OnlineTunnelFrame_ClientPacket:
		pkt := payload.ClientPacket
		if pkt == nil {
			return
		}
		if err := u.remote.Send(buildClientPacketPassThrough(pkt)); err != nil {
			xlog.PrintfErr("user shard[%d]: downstream send failed uid=%d messageID=%d err=%v",
				s.id, uid, pkt.GetMessageId(), err)
		}
	default:
		xlog.PrintfErr("user shard[%d]: unexpected frame payload type for uid=%d", s.id, uid)
	}
}

func buildClientPacketPassThrough(pkt *pb.OnlineClientPacket) *xpacket.PacketPassThrough {
	header := &xpacket.Header{
		Length:    xpacket.HeaderSize + uint32(len(pkt.GetBody())),
		MessageID: pkt.GetMessageId(),
		SessionID: pkt.GetSessionId(),
		ResultID:  pkt.GetResultId(),
		Key:       pkt.GetKey(),
	}
	data := header.Pack()
	copy(data[xpacket.HeaderSize:], pkt.GetBody())
	return &xpacket.PacketPassThrough{
		Header:  header,
		RawData: data,
	}
}
