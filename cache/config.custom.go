package main

import (
	"fmt"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatUserRecord string
var GCfgCustomRedisKeyFormatUserToken string
var GCfgCustomRedisKeyFormatUserSession string

func initCustomConfig() {
	GCfgCustomRedisKeyFormatUserRecord = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserRecord")
	GCfgCustomRedisKeyFormatUserToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserToken")
	GCfgCustomRedisKeyFormatUserSession = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserSession")
}

func RedisKeyUserRecord(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserRecord, uid)
}

func RedisKeyUserToken(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserToken, uid)
}

func RedisKeyUserSession(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserSession, uid)
}
