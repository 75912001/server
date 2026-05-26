package main

import (
	"fmt"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatUserRecord string
var GCfgCustomRedisKeyFormatUserToken string

func initCustomConfig() {
	GCfgCustomRedisKeyFormatUserRecord = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserRecord")
	GCfgCustomRedisKeyFormatUserToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserToken")
}

func RedisKeyUserData(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserRecord, uid)
}

func RedisKeyUserToken(uid uint64, token string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserToken, uid, token)
}
