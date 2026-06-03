package main

import (
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomVerifyExpireTimeDuration time.Duration
var GCfgCustomHeartBeatExpireDuration time.Duration
var GCfgCustomLoginSessionPendingTTL time.Duration

func initCustomConfig() {
	GCfgCustomVerifyExpireTimeDuration = xconfig.GConfigMgr.GetCustomDuration("verifyExpireTimeDuration", 300*time.Second)
	GCfgCustomHeartBeatExpireDuration = xconfig.GConfigMgr.GetCustomDuration("heartBeatExpireTimeDuration", 300*time.Second)
	GCfgCustomLoginSessionPendingTTL = xconfig.GConfigMgr.GetCustomDuration("loginSessionPendingTTL", 30*time.Second)
}
