package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
)

type Account struct {
	uid           uint64
	account       string
	gatewayKey    string
	userSession   string
	clientIP      string
	accountRecord *pb.AccountRecord
	actor         *xactor.Actor[uint64]
}

func newAccount(uid uint64) *Account {
	u := &Account{uid: uid}
	u.actor = xactor.NewActor[uint64](uid, nil, u.behavior)
	u.actor.Start()
	return u
}

func (p *Account) Stop() {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
}
