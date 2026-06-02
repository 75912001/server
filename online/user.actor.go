package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
)

const (
	OnlineUserActorCmdLogin   xactor.CMD = 101
	OnlineUserActorCmdOffline xactor.CMD = 102
)

func (p *User) PostLogin(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineUserActorCmdLogin, req))
	if err != nil {
		return nil, err
	}
	res, _ := resp.(*pb.OnlineUserOnlineRes)
	return res, nil
}

func (p *User) PostOffline() {
	if _, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineUserActorCmdOffline)); err != nil {
		xlog.GLog.Errorf("user offline sync failed uid=%d err=%v", p.uid, err)
		return
	}
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
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
		case OnlineUserActorCmdLogin:
			req, ok := msg.Args[0].(*pb.OnlineUserOnlineReq)
			if !ok {
				continue
			}
			resp, err = p.onLogin(req)
			if err != nil {
				return p.behavior, resp, err
			}
		case OnlineUserActorCmdOffline:
			if err := p.sessionMgr.CleanupOffline(); err != nil {
				xlog.GLog.Warnf("cleanup offline user session failed uid=%d err=%v", p.uid, err)
			}
			GUserMgr.users.Del(p.uid)
		}
	}
	return p.behavior, resp, nil
}
