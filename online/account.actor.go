package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
)

const (
	OnlineAccountActorCmdBind         xactor.CMD = 101
	OnlineAccountActorCmdUnbind       xactor.CMD = 102
	OnlineAccountActorCmdClientPacket xactor.CMD = 103
)

func (p *Account) PostBind(req *pb.OnlineBindUserReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindUserRes, error) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdBind, req, accountRecord))
	if err != nil {
		return nil, err
	}
	res, _ := resp.(*pb.OnlineBindUserRes)
	return res, nil
}

func (p *Account) PostUnbind(gatewayKey string, userSession string) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdUnbind, gatewayKey, userSession))
	if err != nil {
		xlog.GLog.Errorf("account unbind sync failed uid=%d err=%v", p.uid, err)
		return
	}
	stopped, _ := resp.(bool)
	if stopped {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}
}

func (p *Account) PostClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineAccountActorCmdClientPacket, gateway, pkt))
}

func (p *Account) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	var err error
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				if errTmp := event.ICallBack.Execute(); errTmp != nil {
					xlog.GLog.Warnf("account event callback failed uid=%d err=%v", p.uid, errTmp)
				}
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case OnlineAccountActorCmdBind:
			if len(msg.Args) < 2 {
				continue
			}
			req, ok := msg.Args[0].(*pb.OnlineBindUserReq)
			if !ok {
				continue
			}
			accountRecord, ok := msg.Args[1].(*pb.AccountRecord)
			if !ok {
				continue
			}
			resp, err = p.onBind(req, accountRecord)
			if err != nil {
				return p.behavior, resp, err
			}
		case OnlineAccountActorCmdUnbind:
			gatewayKey, ok := msg.Args[0].(string)
			if !ok {
				continue
			}
			userSession, ok := msg.Args[1].(string)
			if !ok {
				continue
			}
			if !p.offlineUserSessionMatch(gatewayKey, userSession) {
				resp = false
				continue
			}
			if currentAccount, ok := GAccountMgr.accounts.Find(p.uid); ok && currentAccount == p {
				GAccountMgr.accounts.Del(p.uid)
			}
			p.gatewayKey = ""
			p.userSession = ""
			resp = true
		case OnlineAccountActorCmdClientPacket:
			gateway, ok := msg.Args[0].(*Gateway)
			if !ok {
				continue
			}
			pkt, ok := msg.Args[1].(*pb.OnlineClientPacket)
			if !ok {
				continue
			}
			p.onClientPacket(gateway, pkt)
		}
	}
	return p.behavior, resp, nil
}

func (p *Account) offlineUserSessionMatch(gatewayKey string, userSession string) bool {
	if gatewayKey == "" || userSession == "" || p.gatewayKey != gatewayKey {
		return false
	}
	return p.userSession == userSession
}
