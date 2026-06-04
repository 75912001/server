package main

import (
	"fmt"
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatUserRecord string
var GCfgCustomRedisKeyFormatUserSession string
var GCfgCustomRedisKeyFormatAccountToken string
var GCfgCustomRedisKeyFormatAccountUID string
var GCfgCustomRedisKeyFormatAccountLock string
var GCfgCustomRedisAccountCreateLockDuration time.Duration
var GCfgBaseGroupID uint32

func initCustomConfig() {
	GCfgBaseGroupID = *xconfig.GConfigMgr.Base.GroupID
	GCfgCustomRedisKeyFormatUserRecord = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserRecord", "user:{%v}:record")
	GCfgCustomRedisKeyFormatUserSession = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserSession", "user:{%v}:session")
	GCfgCustomRedisKeyFormatAccountToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountToken", "account:{%v}:token")
	GCfgCustomRedisKeyFormatAccountUID = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountUID", "account:{%v}:uid")
	GCfgCustomRedisKeyFormatAccountLock = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountLock", "account:{%v}:lock")
	GCfgCustomRedisAccountCreateLockDuration = xconfig.GConfigMgr.GetCustomDuration("redisAccountCreateLockDuration", 5*time.Second)
}

func RedisKeyUserRecord(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserRecord, uid)
}

func RedisKeyUserSession(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserSession, uid)
}

func RedisKeyAccountToken(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountToken, account)
}

func RedisKeyAccountUID(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountUID, account)
}

func RedisKeyAccountLock(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountLock, account)
}

func RedisKeyUserUIDSequence(groupID uint32) string {
	return fmt.Sprintf("user:uid:sequence:{%v}", groupID)
}
