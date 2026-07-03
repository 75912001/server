package main

import (
	"fmt"
	"time"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomRedisKeyFormatAccountRecord string
var GCfgCustomRedisKeyFormatAccountSession string
var GCfgCustomRedisKeyFormatAccountVerifyToken string
var GCfgCustomRedisKeyFormatAccountAID string
var GCfgCustomRedisKeyFormatAccountLock string
var GCfgCustomRedisAccountCreateLockDuration time.Duration
var GCfgBaseGroupID uint32

func initCustomConfig() {
	GCfgBaseGroupID = *xconfig.GConfigMgr.Base.GroupID
	GCfgCustomRedisKeyFormatAccountRecord = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountRecord", "account:{%v}:record")
	GCfgCustomRedisKeyFormatAccountSession = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountSession", "account:{%v}:session")
	GCfgCustomRedisKeyFormatAccountVerifyToken = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountVerifyToken", "account:{%v}:accountVerifyToken")
	GCfgCustomRedisKeyFormatAccountAID = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountAID", "account:{%v}:aid")
	GCfgCustomRedisKeyFormatAccountLock = xconfig.GConfigMgr.GetCustomString("redisKeyFormatAccountLock", "account:{%v}:lock")
	GCfgCustomRedisAccountCreateLockDuration = xconfig.GConfigMgr.GetCustomDuration("redisAccountCreateLockDuration", 5*time.Second)
}

func RedisKeyAccountRecord(aid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountRecord, aid)
}

func RedisKeyAccountSession(aid uint64) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountSession, aid)
}

func RedisKeyAccountVerifyToken(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountVerifyToken, account)
}

func RedisKeyAccountAID(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountAID, account)
}

func RedisKeyAccountLock(account string) string {
	return fmt.Sprintf(GCfgCustomRedisKeyFormatAccountLock, account)
}

func RedisKeyAccountAIDSequence(groupID uint32) string {
	return fmt.Sprintf("account:aid:sequence:{%v}", groupID)
}
