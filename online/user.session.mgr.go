package main

import (
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
	xtimer "github.com/75912001/xlib/timer"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type userSessionMgr struct {
	user        *User
	session     *cacheUserSession
	expireTimer *xtimer.Second
}

func newUserSessionMgr(user *User) *userSessionMgr {
	return &userSessionMgr{user: user}
}

func (p *userSessionMgr) Bind(session *cacheUserSession) {
	p.session = session
	p.startExpireTimer()
}

func (p *userSessionMgr) Stop() {
	p.stopExpireTimer()
}

func (p *userSessionMgr) CleanupOffline() error {
	p.stopExpireTimer()
	if p.session == nil {
		return nil
	}
	err := unaryCacheDelUserSession(p.user.uid, p.session)
	p.session = nil
	return err
}

func (p *userSessionMgr) startExpireTimer() {
	p.stopExpireTimer()
	cb := xcontrol.NewCallBack(func(args ...any) error {
		return p.onExpireTimer()
	})
	p.expireTimer = xtimer.GTimer.AddSecond(cb, time.Now().Unix()+int64(userSessionRefreshSecond), p.user.actor)
}

func (p *userSessionMgr) stopExpireTimer() {
	if p.expireTimer == nil {
		return
	}
	xtimer.GTimer.DelSecond(p.expireTimer)
	p.expireTimer = nil
}

func (p *userSessionMgr) onExpireTimer() error {
	p.expireTimer = nil
	if p.session == nil {
		return nil
	}
	if err := unaryCacheSetUserSessionExpire(p.user.uid, p.session); err != nil {
		if s, ok := grpcstatus.FromError(err); ok && (s.Code() == grpccodes.Aborted || s.Code() == grpccodes.NotFound) {
			p.session = nil
			return nil
		}
		xlog.GLog.Warnf("refresh user session expire failed uid=%d err=%v", p.user.uid, err)
	}
	if p.session != nil {
		p.startExpireTimer()
	}
	return nil
}
