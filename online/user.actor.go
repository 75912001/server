package main

import (
	"context"
	"fmt"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	OnlineUserActorCmdKickOldGateway xactor.CMD = 100
	OnlineUserActorCmdLogin          xactor.CMD = 101
	OnlineUserActorCmdOffline        xactor.CMD = 102
)

func (p *User) PostLogin(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	if _, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineUserActorCmdKickOldGateway)); err != nil {
		return nil, err
	}
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
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case OnlineUserActorCmdKickOldGateway:
			if p.gatewayID != "" {
				if err := p.kickOldGateway(); err != nil {
					xlog.PrintfErr("kick old gateway failed uid=%d gateway=%s err=%v", p.uid, p.gatewayID, err)
					return p.behavior, resp, status.Error(codes.FailedPrecondition, fmt.Sprintf("kick old gateway failed: %v", err))
				}
			}
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
			GUserMgr.users.Del(p.uid)
		}
	}
	return p.behavior, resp, nil
}
