package main

import (
	"fmt"
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatAccountRecord string
var GCfgCustomRedisKeyFormatUserSession string
var GCfgCustomRedisKeyFormatAccountVerifyToken string
var GCfgCustomRedisKeyFormatAccountUID string
var GCfgCustomRedisKeyFormatAccountLock string
var GCfgCustomRedisAccountCreateLockDuration time.Duration
var GCfgBaseGroupID uint32

func initCustomConfig() {
	GCfgBaseGroupID = *xconfig.GConfigMgr.Base.GroupID
	GCfgCustomRedisKeyFormatAccountRecord = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountRecord", "account:{%v}:record")
	GCfgCustomRedisKeyFormatUserSession = xconfig.GConfigMgr.GetCustomString("redisKeyFormatUserSession", "user:{%v}:session")
	GCfgCustomRedisKeyFormatAccountVerifyToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountVerifyToken", "account:{%v}:accountVerifyToken")
	GCfgCustomRedisKeyFormatAccountUID = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountUID", "account:{%v}:uid")
	GCfgCustomRedisKeyFormatAccountLock = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountLock", "account:{%v}:lock")
	GCfgCustomRedisAccountCreateLockDuration = xconfig.GConfigMgr.GetCustomDuration("redisAccountCreateLockDuration", 5*time.Second)
}

func RedisKeyAccountRecord(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountRecord, uid)
}

func RedisKeyUserSession(uid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatUserSession, uid)
}

func RedisKeyAccountVerifyToken(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountVerifyToken, account)
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
