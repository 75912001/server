package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// UserActorCmdOnlineTunnelFrame online 下行给当前用户的业务包
const UserActorCmdOnlineTunnelFrame xactor.CMD = 100

func (p *User) PostFrame(frame *pb.OnlineTunnelFrame) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), UserActorCmdOnlineTunnelFrame, frame))
}

// UserActorCmdUserVerified 登录验证成功后操作, 绑定 uid、online，并启动心跳定时器
const UserActorCmdUserVerified xactor.CMD = 101

func (p *User) PostSyncVerified(uid uint64, account string, online *Online, gatewaySession string, userSession string) error {
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), UserActorCmdUserVerified, uid, account, online, gatewaySession, userSession))
	if err != nil {
		return errors.WithMessagef(err, "user verified sync failed uid:%v online:%v %v", uid, online, xruntime.Location())
	}
	return nil
}

// UserActorCmdUserPacket 客户端上行包client->gateway，包含心跳、主动离线和业务透传
const UserActorCmdUserPacket xactor.CMD = 102

func (p *User) PostClientPacket(header *xpacket.Header, body []byte) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), UserActorCmdUserPacket, header, body))
}

// UserActorCmdUserCleanup 清理用户
const UserActorCmdUserCleanup xactor.CMD = 103

func (p *User) PostSyncCleanup(reason xnetcommon.DisconnectReason) {
	defer func() {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}()
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), UserActorCmdUserCleanup, reason))
	if err != nil {
		xlog.GLog.Errorf("user cleanup sync failed remote=%p err=%v", p.remote, err)
		return
	}
}

func (p *User) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	var err error
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				errTmp := event.ICallBack.Execute()
				err = xerror.AppendError(err, errors.WithMessagef(errTmp, "user event callback error %v", xruntime.Location()))
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
			account, ok := msg.Args[1].(string)
			if !ok {
				continue
			}
			online, ok := msg.Args[2].(*Online)
			if !ok {
				continue
			}
			gatewaySession, ok := msg.Args[3].(string)
			if !ok {
				continue
			}
			userSession, ok := msg.Args[4].(string)
			if !ok {
				continue
			}
			if err := p.OnVerified(uid, account, online, gatewaySession, userSession); err != nil {
				return p.behavior, resp, errors.WithMessagef(err, "user verified failed uid:%v online:%v %v", uid, online, xruntime.Location())
			}
		case UserActorCmdUserPacket:
			header, ok := msg.Args[0].(*xpacket.Header)
			if !ok {
				continue
			}
			body, ok := msg.Args[1].([]byte)
			if !ok {
				continue
			}
			errTmp := p.OnClientPacket(header, body)
			err = xerror.AppendError(err, errors.WithMessagef(errTmp, "user packet error %v", xruntime.Location()))
		case UserActorCmdUserCleanup:
			reason, ok := msg.Args[0].(xnetcommon.DisconnectReason)
			if ok {
				p.Cleanup(reason)
			}
		}
	}
	return p.behavior, resp, err
}
