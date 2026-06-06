package main

import (
	"time"

	"server/common"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomVerifyExpireTimeDuration time.Duration
var GCfgCustomHeartBeatExpireDuration time.Duration
var GCfgCustomTicketSecret string

func initCustomConfig() {
	GCfgCustomVerifyExpireTimeDuration = xconfig.GConfigMgr.GetCustomDuration("verifyExpireTimeDuration", 300*time.Second)
	GCfgCustomHeartBeatExpireDuration = xconfig.GConfigMgr.GetCustomDuration("heartBeatExpireTimeDuration", 300*time.Second)
	GCfgCustomTicketSecret = xconfig.GConfigMgr.GetCustomString("ticketSecret", common.ConnectTicketSecretDefault)
}
