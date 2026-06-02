package main

import (
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xtimer "github.com/75912001/xlib/timer"
)

func (p *Client) SendHeartBeat() {
	p.heartbeatOnce.Do(func() {
		p.startHeartBeatTimer()
	})
}

func (p *Client) startHeartBeatTimer() {
	if xtimer.GTimer == nil || p.iEventMgr == nil {
		return
	}
	if p.heartbeatTimer != nil {
		xtimer.GTimer.DelMillisecond(p.heartbeatTimer)
		p.heartbeatTimer = nil
	}
	cb := xcontrol.NewCallBack(func(args ...any) error {
		p.heartbeatTimer = nil
		if p.isClosed() || !p.verified || p.Remote == nil || !p.Remote.IsConnect() {
			return nil
		}
		p.iEventMgr.Send(&EventCommand{Command: "UserHeartbeatReq"})
		p.startHeartBeatTimer()
		return nil
	})
	p.heartbeatTimer = xtimer.GTimer.AddMillisecond(cb, time.Now().Add(GConfigYaml.HeartbeatInterval).UnixMilli(), p.iEventMgr)
}

func (p *Client) isClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}
