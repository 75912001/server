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
	// UserActorCmdOnlineTunnelFrame 参数：*pb.OnlineTunnelFrame；online 下行给当前用户的业务包。
	UserActorCmdOnlineTunnelFrame xactor.CMD = 100
	// UserActorCmdUserVerified 参数：uid uint64, online *Online；登录成功后绑定 uid、online，并启动心跳定时器。
	UserActorCmdUserVerified xactor.CMD = 101
	// UserActorCmdUserPacket 参数：header *xpacket.Header, body []byte；处理客户端上行包，包含心跳、主动离线和业务透传。
	UserActorCmdUserPacket xactor.CMD = 102
	// UserActorCmdUserCleanup 参数：xnetcommon.DisconnectReason；连接断开后清理用户定时器和状态，返回 uid。
	UserActorCmdUserCleanup xactor.CMD = 103
)

type userPacketEvent struct {
	header *xpacket.Header
	body   []byte
}

func (u *User) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
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
		case UserActorCmdOnlineTunnelFrame:
			frame, ok := msg.Args[0].(*pb.OnlineTunnelFrame)
			if ok {
				u.handleOnlineFrame(frame)
			}
		case UserActorCmdUserVerified:
			uid, ok := msg.Args[0].(uint64)
			if !ok {
				continue
			}
			online, ok := msg.Args[1].(*Online)
			if !ok {
				continue
			}
			u.OnVerified(uid, online)
		case UserActorCmdUserPacket:
			header, ok := msg.Args[0].(*xpacket.Header)
			if !ok {
				continue
			}
			body, ok := msg.Args[1].([]byte)
			if !ok {
				continue
			}
			_ = u.OnClientPacket(header, body)
		case UserActorCmdUserCleanup:
			reason, ok := msg.Args[0].(xnetcommon.DisconnectReason)
			if ok {
				resp = u.Cleanup(reason)
			}
		}
	}
	return u.behavior, resp, nil
}

func (u *User) handleOnlineFrame(frame *pb.OnlineTunnelFrame) {
	if !u.remote.IsConnect() {
		return
	}
	uid := frame.GetUid()
	if uid != u.uid {
		xlog.PrintfErr("user actor uid mismatch: actor uid=%d frame uid=%d", u.uid, uid)
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
			xlog.PrintfErr("user downstream send failed uid=%d messageID=%d err=%v",
				uid, pkt.GetMessageId(), err)
		}
	default:
		xlog.PrintfErr("unexpected frame payload type for uid=%d", uid)
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
