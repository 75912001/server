package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
)

const (
	OnlineUserActorCmdBind         xactor.CMD = 101
	OnlineUserActorCmdUnbind       xactor.CMD = 102
	OnlineUserActorCmdClientPacket xactor.CMD = 103
)

func (p *User) PostBind(req *pb.OnlineBindUserReq, userRecord *pb.UserRecord) (*pb.OnlineBindUserRes, error) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineUserActorCmdBind, req, userRecord))
	if err != nil {
		return nil, err
	}
	res, _ := resp.(*pb.OnlineBindUserRes)
	return res, nil
}

func (p *User) PostUnbind(gatewayKey string, userSession string) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineUserActorCmdUnbind, gatewayKey, userSession))
	if err != nil {
		xlog.GLog.Errorf("user unbind sync failed uid=%d err=%v", p.uid, err)
		return
	}
	stopped, _ := resp.(bool)
	if stopped {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}
}

func (p *User) PostClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineUserActorCmdClientPacket, gateway, pkt))
}

func (p *User) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	var err error
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				if errTmp := event.ICallBack.Execute(); errTmp != nil {
					xlog.GLog.Warnf("user event callback failed uid=%d err=%v", p.uid, errTmp)
				}
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case OnlineUserActorCmdBind:
			if len(msg.Args) < 2 {
				continue
			}
			req, ok := msg.Args[0].(*pb.OnlineBindUserReq)
			if !ok {
				continue
			}
			userRecord, ok := msg.Args[1].(*pb.UserRecord)
			if !ok {
				continue
			}
			resp, err = p.onBind(req, userRecord)
			if err != nil {
				return p.behavior, resp, err
			}
		case OnlineUserActorCmdUnbind:
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
			if currentUser, ok := GUserMgr.users.Find(p.uid); ok && currentUser == p {
				GUserMgr.users.Del(p.uid)
			}
			p.gatewayID = ""
			p.userSession = ""
			resp = true
		case OnlineUserActorCmdClientPacket:
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

func (p *User) offlineUserSessionMatch(gatewayKey string, userSession string) bool {
	if gatewayKey == "" || userSession == "" || p.gatewayID != gatewayKey {
		return false
	}
	return p.userSession == userSession
}
