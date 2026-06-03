package main

import (
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomHTTPAddr string
var GCfgCustomTokenPath string
var GCfgCustomTokenExpireSecond uint64
var GCfgCustomReadHeaderTimeout time.Duration
var GCfgCustomShutdownTimeout time.Duration
var GCfgCustomCacheRPCTimeout time.Duration
var GCfgCustomMaxBodyBytes int64

func initCustomConfig() {
	GCfgCustomHTTPAddr = xconfig.GConfigMgr.GetCustomString("httpAddr")
	GCfgCustomTokenPath = xconfig.GConfigMgr.GetCustomString("tokenPath", "/api/login/token")
	GCfgCustomTokenExpireSecond = uint64(xconfig.GConfigMgr.GetCustomDuration("tokenExpireSecond", 10*time.Second) / time.Second)
	GCfgCustomReadHeaderTimeout = xconfig.GConfigMgr.GetCustomDuration("readHeaderTimeout", 5*time.Second)
	GCfgCustomShutdownTimeout = xconfig.GConfigMgr.GetCustomDuration("shutdownTimeout", 10*time.Second)
	GCfgCustomCacheRPCTimeout = xconfig.GConfigMgr.GetCustomDuration("cacheRPCTimeout", 3*time.Second)
	GCfgCustomMaxBodyBytes = xconfig.GConfigMgr.GetCustomInt64("maxBodyBytes", 4096)
}
