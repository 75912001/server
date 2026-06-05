package main

import (
	"time"

	"server/common"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomHTTPAddr string
var GCfgCustomTokenPath string
var GCfgCustomSessionPath string
var GCfgCustomTokenExpireSecond uint64
var GCfgCustomTicketExpireSecond uint64
var GCfgCustomTicketSecret string
var GCfgCustomReadHeaderTimeout time.Duration
var GCfgCustomShutdownTimeout time.Duration
var GCfgCustomCacheRPCTimeout time.Duration
var GCfgCustomMaxBodyBytes int64

func initCustomConfig() {
	GCfgCustomHTTPAddr = xconfig.GConfigMgr.GetCustomString("httpAddr")
	GCfgCustomTokenPath = xconfig.GConfigMgr.GetCustomString("tokenPath", "/api/login/token")
	GCfgCustomSessionPath = xconfig.GConfigMgr.GetCustomString("sessionPath", "/api/login/session")
	GCfgCustomTokenExpireSecond = uint64(xconfig.GConfigMgr.GetCustomDuration("tokenExpireSecond", 10*time.Second) / time.Second)
	GCfgCustomTicketExpireSecond = uint64(xconfig.GConfigMgr.GetCustomDuration("ticketExpireSecond", 10*time.Second) / time.Second)
	GCfgCustomTicketSecret = xconfig.GConfigMgr.GetCustomString("ticketSecret", common.ConnectTicketSecretDefault)
	GCfgCustomReadHeaderTimeout = xconfig.GConfigMgr.GetCustomDuration("readHeaderTimeout", 5*time.Second)
	GCfgCustomShutdownTimeout = xconfig.GConfigMgr.GetCustomDuration("shutdownTimeout", 10*time.Second)
	GCfgCustomCacheRPCTimeout = xconfig.GConfigMgr.GetCustomDuration("cacheRPCTimeout", 3*time.Second)
	GCfgCustomMaxBodyBytes = xconfig.GConfigMgr.GetCustomInt64("maxBodyBytes", 4096)
}
