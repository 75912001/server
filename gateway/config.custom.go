package main

import (
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomverifyExpireTimeDuration time.Duration
var GCfgCustomHeartBeatExpireDuration time.Duration

func initCustomConfig() {
	GCfgCustomverifyExpireTimeDuration = xconfig.GConfigMgr.GetCustomDuration("verifyExpireTimeDuration", 300*time.Second)
	GCfgCustomHeartBeatExpireDuration = xconfig.GConfigMgr.GetCustomDuration("heartBeatExpireTimeDuration", 300*time.Second)
}

func cfgHeartBeatExpireSecond() int64 {
	return int64(GCfgCustomHeartBeatExpireDuration.Seconds())
}
