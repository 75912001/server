package main

import (
	"context"
	"math/rand"
	pb "server/proto/pb"
	"time"

	xactor "github.com/75912001/xlib/actor"
	xtimer "github.com/75912001/xlib/timer"
)

type Account struct {
	aid            uint64
	account        string
	gatewayKey     string
	accountSession string
	clientIP       string
	accountRecord  *pb.AccountRecord
	actor          *xactor.Actor[uint64]

	onlineCharacterUUIDSet map[uint64]struct{}
	activeCharacterUUID    uint64

	autoEncounterEnabled  bool
	autoEncounterTimer    *xtimer.Second
	autoEncounterTimerSeq uint64
	roundTimer            *xtimer.Second
	roundTimerSeq         uint64
	combatState           *accountCombatState
	lastBattleID          uint64
	rng                   *rand.Rand
}

func newAccount(aid uint64) *Account {
	u := &Account{
		aid:                    aid,
		onlineCharacterUUIDSet: make(map[uint64]struct{}),
		rng:                    rand.New(rand.NewSource(time.Now().UnixNano() + int64(aid))),
	}
	u.actor = xactor.NewActor[uint64](aid, nil, u.behavior)
	u.actor.Start()
	return u
}

func (p *Account) Stop() {
	p.clearCombatRuntime()
	p.persistOnlineCharacterLogoutTimestamp(time.Now().UnixMilli())
	p.clearOnlineCharacterUUIDs()
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
}
