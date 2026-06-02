package main

import (
	"sync"

	xevent "github.com/75912001/xlib/event"
	xnetcommon "github.com/75912001/xlib/net/common"
	xnettcp "github.com/75912001/xlib/net/tcp"
	xtimer "github.com/75912001/xlib/timer"
)

type Client struct {
	TCP    *xnettcp.Client
	Remote xnetcommon.IRemote

	iEventMgr *xevent.ListMgr

	uid            uint64
	token          string
	nextSession    uint32
	verified       bool
	heartbeatTimer *xtimer.Millisecond
	ignoreMsgID    map[uint32]struct{}
	heartbeatOnce  sync.Once
	closeOnce      sync.Once
	closed         chan struct{}
}

var clientOnce sync.Once
var client *Client

func GetClient() *Client {
	clientOnce.Do(func() {
		client = &Client{
			iEventMgr: xevent.NewListMgr(1, Bus),
			closed:    make(chan struct{}),
		}
	})
	return client
}

func (p *Client) Close() {
	p.closeOnce.Do(func() {
		p.verified = false
		if p.Remote != nil && p.Remote.IsConnect() {
			p.Remote.Stop()
		}
		if xtimer.GTimer != nil && p.heartbeatTimer != nil {
			xtimer.GTimer.DelMillisecond(p.heartbeatTimer)
			p.heartbeatTimer = nil
		}
		if p.iEventMgr != nil {
			p.iEventMgr.Stop()
		}
		close(p.closed)
	})
}
