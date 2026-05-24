package main

import xconfig "github.com/75912001/xlib/config"

const (
	GatewayCustomKeyVerifyExpireTimeSecond = "verifyExpireTimeSecond"
	GatewayCustomKeyHeartBeatExpireSecond  = "heartBeatExpireTimeSecond"
	GatewayCustomKeyUserActorCount         = "userActorCount"
)

const (
	GatewayDefaultVerifyExpireTimeSecond int64 = 86400
	GatewayDefaultHeartBeatExpireSecond  int64 = 60
	GatewayDefaultUserActorCount         int64 = 64
)

func cfgVerifyExpireTimeSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyVerifyExpireTimeSecond, GatewayDefaultVerifyExpireTimeSecond)
}

func cfgHeartBeatExpireSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyHeartBeatExpireSecond, GatewayDefaultHeartBeatExpireSecond)
}

func cfgUserActorCount() int {
	return int(xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyUserActorCount, GatewayDefaultUserActorCount))
}
