package main

import (
	"fmt"
	"sync/atomic"
)

type RobotStats struct {
	connectOK    atomic.Uint64
	connectFail  atomic.Uint64
	disconnect   atomic.Uint64
	verifyOK     atomic.Uint64
	verifyFail   atomic.Uint64
	sent         atomic.Uint64
	received     atomic.Uint64
	sendFail     atomic.Uint64
	resultFail   atomic.Uint64
	queued       atomic.Uint64
	actionSent   atomic.Uint64
	actionSkip   atomic.Uint64
	commandError atomic.Uint64
}

type RobotStatsSnapshot struct {
	ConnectOK    uint64
	ConnectFail  uint64
	Disconnect   uint64
	VerifyOK     uint64
	VerifyFail   uint64
	Sent         uint64
	Received     uint64
	SendFail     uint64
	ResultFail   uint64
	Queued       uint64
	ActionSent   uint64
	ActionSkip   uint64
	CommandError uint64
}

func (p *RobotStats) Snapshot() RobotStatsSnapshot {
	return RobotStatsSnapshot{
		ConnectOK:    p.connectOK.Load(),
		ConnectFail:  p.connectFail.Load(),
		Disconnect:   p.disconnect.Load(),
		VerifyOK:     p.verifyOK.Load(),
		VerifyFail:   p.verifyFail.Load(),
		Sent:         p.sent.Load(),
		Received:     p.received.Load(),
		SendFail:     p.sendFail.Load(),
		ResultFail:   p.resultFail.Load(),
		Queued:       p.queued.Load(),
		ActionSent:   p.actionSent.Load(),
		ActionSkip:   p.actionSkip.Load(),
		CommandError: p.commandError.Load(),
	}
}

func (p RobotStatsSnapshot) String(total int) string {
	return formatRobotStats(total, p)
}

func formatRobotStats(total int, p RobotStatsSnapshot) string {
	active := uint64(0)
	if p.ConnectOK > p.Disconnect {
		active = p.ConnectOK - p.Disconnect
	}
	return fmt.Sprintf("robots=%d active=%d connectOK=%d connectFail=%d verifyOK=%d verifyFail=%d disconnect=%d sent=%d recv=%d sendFail=%d resultFail=%d queued=%d actionSent=%d actionSkip=%d commandError=%d",
		total, active, p.ConnectOK, p.ConnectFail, p.VerifyOK, p.VerifyFail, p.Disconnect, p.Sent, p.Received, p.SendFail, p.ResultFail, p.Queued, p.ActionSent, p.ActionSkip, p.CommandError)
}
