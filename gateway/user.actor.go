package main

import (
	"context"
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

func (p *User) PostFrame(frame *pb.OnlineTunnelFrame) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), UserActorCmdOnlineTunnelFrame, frame))
}

func (p *User) PostSyncVerified(uid uint64, online *Online) error {
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), UserActorCmdUserVerified, uid, online))
	return err
}

func (p *User) PostClientPacket(header *xpacket.Header, body []byte) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), UserActorCmdUserPacket, header, body))
}

func (p *User) PostSyncCleanup(reason xnetcommon.DisconnectReason) uint64 {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), UserActorCmdUserCleanup, reason))
	if err != nil {
		xlog.PrintfErr("user cleanup sync failed remote=%p err=%v", p.remote, err)
		return 0
	}
	uid, _ := resp.(uint64)
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	return uid
}

func (p *User) behavior(messages ...any) (xactor.Behavior, any, error) {
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
				p.handleOnlineFrame(frame)
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
			p.OnVerified(uid, online)
		case UserActorCmdUserPacket:
			header, ok := msg.Args[0].(*xpacket.Header)
			if !ok {
				continue
			}
			body, ok := msg.Args[1].([]byte)
			if !ok {
				continue
			}
			_ = p.OnClientPacket(header, body)
		case UserActorCmdUserCleanup:
			reason, ok := msg.Args[0].(xnetcommon.DisconnectReason)
			if ok {
				resp = p.Cleanup(reason)
			}
		}
	}
	return p.behavior, resp, nil
}
