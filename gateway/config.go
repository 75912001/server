package main

import xconfig "github.com/75912001/xlib/config"

const (
	GatewayCustomKeyVerifyExpireTimeSecond = "verifyExpireTimeSecond"
	GatewayCustomKeyHeartBeatExpireSecond  = "heartBeatExpireTimeSecond"
)

const (
	GatewayDefaultVerifyExpireTimeSecond int64 = 300
	GatewayDefaultHeartBeatExpireSecond  int64 = 300
)

func cfgVerifyExpireTimeSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyVerifyExpireTimeSecond, GatewayDefaultVerifyExpireTimeSecond)
}

func cfgHeartBeatExpireSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyHeartBeatExpireSecond, GatewayDefaultHeartBeatExpireSecond)
}
