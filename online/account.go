package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
)

type Account struct {
	aid            uint64            // 账号 id
	account        string            // 账号
	gatewayKey     string            // gateway key
	accountSession string            // 账号 session
	clientIP       string            // 客户端 ip
	accountRecord  *pb.AccountRecord // 账号 记录
	actor          *xactor.Actor[uint64]

	characterManager *characterMgr // 账号内全部角色的在线, 自动遇敌和 CombatRoom actor 指针
}

func newAccount(aid uint64) *Account {
	u := &Account{aid: aid}
	u.characterManager = newCharacterMgr(u, nil)
	u.actor = xactor.NewActor[uint64](aid, nil, u.behavior)
	u.actor.Start()
	return u
}

func (p *Account) Stop() {
	if _, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdStop)); err != nil {
		xlog.GLog.Errorf("account stop cleanup failed aid:%d err:%v", p.aid, err)
	}
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
}
