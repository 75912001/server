package main

import (
	"fmt"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatUserData string
var GCfgCustomRedisKeyFormatUserToken string

func initCustomConfig() {
	GCfgCustomRedisKeyFormatUserData = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserData")
	GCfgCustomRedisKeyFormatUserToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserToken")
}

func RedisKeyUserData(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserData, uid)
}

func RedisKeyUserToken(uid uint64, token string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserToken, uid, token)
}
