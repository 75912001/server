package main

import (
	"math/rand"
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xtimer "github.com/75912001/xlib/timer"
)

func (p *Robot) StartHeartBeat() {
	p.startHeartBeatTimer()
}

func (p *Robot) startHeartBeatTimer() {
	if xtimer.GTimer == nil || p.manager == nil || p.manager.iEventMgr == nil {
		return
	}
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelMillisecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}
	p.heartbeatTimerSeq++
	timerSeq := p.heartbeatTimerSeq
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if timerSeq != p.heartbeatTimerSeq {
			return nil
		}
		p.heartbeatTimer = nil
		if p.isClosed() || !p.verified || p.Remote == nil || !p.Remote.IsConnect() {
			return nil
		}
		p.manager.iEventMgr.Send(&RobotCommand{Robot: p, Command: "UserHeartbeatReq", Source: "heartbeat"})
		return nil
	})
	p.heartbeatTimer = xtimer.GTimer.AddMillisecond(cb, time.Now().Add(GConfigYaml.Robot.HeartbeatInterval).UnixMilli(), p.manager.iEventMgr)
}

func (p *Robot) StartAction() {
	p.startActionTimer()
}

func (p *Robot) startActionTimer() {
	if xtimer.GTimer == nil || p.manager == nil || p.manager.iEventMgr == nil {
		return
	}
	if GConfigYaml.Robot.ActionInterval <= 0 || len(GConfigYaml.Robot.Messages) == 0 {
		return
	}
	if p.actionTimer != nil {
		xtimer.GTimer.DelMillisecond(p.actionTimer)
		p.actionTimer = nil
	}
	next := GConfigYaml.Robot.ActionInterval
	if GConfigYaml.Robot.ActionJitter > 0 {
		next += time.Duration(rand.Int63n(int64(GConfigYaml.Robot.ActionJitter)))
	}
	cb := xcontrol.NewCallBack(func(args ...any) error {
		p.actionTimer = nil
		if p.isClosed() || !p.verified || p.Remote == nil || !p.Remote.IsConnect() {
			return nil
		}
		command := selectRobotActionMessage()
		if command == "" {
			p.manager.stats.actionSkip.Add(1)
			p.startActionTimer()
			return nil
		}
		p.manager.iEventMgr.Send(&RobotCommand{Robot: p, Command: command, Source: "action"})
		p.startActionTimer()
		return nil
	})
	p.actionTimer = xtimer.GTimer.AddMillisecond(cb, time.Now().Add(next).UnixMilli(), p.manager.iEventMgr)
}

func selectRobotActionMessage() string {
	total := 0
	for _, item := range GConfigYaml.Robot.Messages {
		if item.Weight > 0 && item.Name != "" {
			total += item.Weight
		}
	}
	if total <= 0 {
		return ""
	}
	target := rand.Intn(total)
	current := 0
	for _, item := range GConfigYaml.Robot.Messages {
		if item.Weight <= 0 || item.Name == "" {
			continue
		}
		current += item.Weight
		if target < current {
			return item.Name
		}
	}
	return ""
}
