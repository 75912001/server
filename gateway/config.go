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

func gatewayVerifyExpireTimeSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyVerifyExpireTimeSecond, GatewayDefaultVerifyExpireTimeSecond)
}

func gatewayHeartBeatExpireSecond() int64 {
	return xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyHeartBeatExpireSecond, GatewayDefaultHeartBeatExpireSecond)
}

func gatewayUserActorCount() int {
	return int(xconfig.GConfigMgr.GetCustomInt64(GatewayCustomKeyUserActorCount, GatewayDefaultUserActorCount))
}
