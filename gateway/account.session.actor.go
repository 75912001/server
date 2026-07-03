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

// AccountActorCmdOnlineTunnelFrame online 下行给当前用户的业务包
const AccountActorCmdOnlineTunnelFrame xactor.CMD = 100

func (p *Account) PostFrame(frame *pb.OnlineTunnelFrame) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), AccountActorCmdOnlineTunnelFrame, frame))
}

// AccountActorCmdAccountVerified 登录验证成功后操作, 绑定 aid、online，并启动心跳定时器
const AccountActorCmdAccountVerified xactor.CMD = 101

func (p *Account) PostSyncVerified(aid uint64, account string, online *Online, heartbeatSession string, accountSession string) error {
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), AccountActorCmdAccountVerified, aid, account, online, heartbeatSession, accountSession))
	if err != nil {
		return errors.WithMessagef(err, "account verified sync failed aid:%v online:%v %v", aid, online, xruntime.Location())
	}
	return nil
}

// AccountActorCmdAccountPacket 客户端上行包client->gateway，包含心跳、主动离线和业务透传
const AccountActorCmdAccountPacket xactor.CMD = 102

func (p *Account) PostClientPacket(header *xpacket.Header, body []byte) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), AccountActorCmdAccountPacket, header, body))
}

// AccountActorCmdAccountCleanup 清理用户
const AccountActorCmdAccountCleanup xactor.CMD = 103

func (p *Account) PostSyncCleanup(reason xnetcommon.DisconnectReason) error {
	defer func() {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}()
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), AccountActorCmdAccountCleanup, reason))
	if err != nil {
		return errors.WithMessagef(err, "account cleanup sync failed remote=%p", p.remote)
	}
	return nil
}

func (p *Account) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	var err error
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				errTmp := event.ICallBack.Execute()
				err = xerror.AppendError(err, errors.WithMessagef(errTmp, "account event callback error %v", xruntime.Location()))
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case AccountActorCmdOnlineTunnelFrame:
			frame, ok := msg.Args[0].(*pb.OnlineTunnelFrame)
			if ok {
				p.handleOnlineFrame(frame)
			}
		case AccountActorCmdAccountVerified:
			aid, ok := msg.Args[0].(uint64)
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
			heartbeatSession, ok := msg.Args[3].(string)
			if !ok {
				continue
			}
			accountSession, ok := msg.Args[4].(string)
			if !ok {
				continue
			}
			if err := p.OnVerified(aid, account, online, heartbeatSession, accountSession); err != nil {
				return p.behavior, resp, errors.WithMessagef(err, "account verified failed aid:%v online:%v %v", aid, online, xruntime.Location())
			}
		case AccountActorCmdAccountPacket:
			header, ok := msg.Args[0].(*xpacket.Header)
			if !ok {
				continue
			}
			body, ok := msg.Args[1].([]byte)
			if !ok {
				continue
			}
			errTmp := p.OnClientPacket(header, body)
			err = xerror.AppendError(err, errors.WithMessagef(errTmp, "account packet error %v", xruntime.Location()))
		case AccountActorCmdAccountCleanup:
			reason, ok := msg.Args[0].(xnetcommon.DisconnectReason)
			if ok {
				if errTmp := p.Cleanup(reason); errTmp != nil {
					xlog.GLog.Errorf("account cleanup failed aid=%d err=%v", p.aid, errTmp)
					err = xerror.AppendError(err, errTmp)
				}
			}
		}
	}
	return p.behavior, resp, err
}
