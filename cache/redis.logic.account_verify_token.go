package main

import (
	"context"
	"server/common"
	"server/proto/pb"
	"strconv"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// SetAccountVerifyToken 写入 accountVerifyToken。
// 使用 SETNX 保证未消费的 accountVerifyToken 不会被覆盖, 返回 false 表示 key 已存在。
func (p *Redis) SetAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string, expire time.Duration) (bool, error) {
	key := RedisKeyAccountVerifyToken(account)
	return p.client.SetNX(ctx, key, accountVerifyToken, expire).Result()
}

// useAccountVerifyTokenScript 原子校验并消费 accountVerifyToken。
// 成功时删除 accountVerifyToken key; key 不存在或 accountVerifyToken 不匹配时返回 0。
const useAccountVerifyTokenScript = `
local current = redis.call("GET", KEYS[1])
if current == false then
	return 0
end
if current ~= ARGV[1] then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`

// UseAccountVerifyToken 验证并消费 accountVerifyToken。
// 消费成功后删除 accountVerifyToken key, 避免同一 accountVerifyToken 被重复使用。
func (p *Redis) UseAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string) (bool, error) {
	key := RedisKeyAccountVerifyToken(account)
	result, err := p.client.Eval(ctx, useAccountVerifyTokenScript, []string{key}, accountVerifyToken).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "use accountVerifyToken from redis failed, account: %s, accountVerifyToken: %s %v", account, accountVerifyToken, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

// EnsureAccount 确保 account 有唯一 uid 和 AccountRecord。
// 返回值 created 表示本次调用是否新建了账号。
func (p *Redis) EnsureAccount(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	for {
		accountRecord, found, err := p.GetAccountRecordByAccount(ctx, account)
		if err != nil || found {
			return accountRecord, false, err
		}

		locked, err := p.client.SetNX(ctx, RedisKeyAccountLock(account), "1", GCfgCustomRedisAccountCreateLockDuration).Result()
		if err != nil {
			return nil, false, errors.WithMessagef(err, "lock account create failed, account: %s %v", account, xruntime.Location())
		}
		if !locked {
			if err = sleepContext(ctx, 20*time.Millisecond); err != nil {
				return nil, false, err
			}
			continue
		}

		accountRecord, created, err := p.createAccountAfterLock(ctx, account)
		if _, unlockErr := p.client.Del(ctx, RedisKeyAccountLock(account)).Result(); unlockErr != nil && err == nil {
			err = errors.WithMessagef(unlockErr, "unlock account create failed, account: %s %v", account, xruntime.Location())
		}
		return accountRecord, created, err
	}
}

// createAccountAfterLock 在持有账号创建锁后创建账号数据。
// 创建前会再次查询账号映射，避免等待锁期间其他请求已经完成创建。
func (p *Redis) createAccountAfterLock(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	accountRecord, found, err := p.GetAccountRecordByAccount(ctx, account)
	if err != nil || found {
		return accountRecord, false, err
	}

	groupID := GCfgBaseGroupID
	startUID := common.GroupUIDStart(groupID)
	sequenceKey := RedisKeyUserUIDSequence(groupID)
	if err = p.client.SetNX(ctx, sequenceKey, startUID-1, 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "init uid sequence failed, account: %s %v", account, xruntime.Location())
	}
	uid, err := p.client.Incr(ctx, sequenceKey).Uint64()
	if err != nil {
		return nil, false, errors.WithMessagef(err, "incr uid sequence failed, account: %s %v", account, xruntime.Location())
	}

	now := time.Now().UnixMilli()
	accountRecord = &pb.AccountRecord{
		Uid:                            uid,
		Account:                        account,
		AccountCreateTimestampMs:       now,
		AccountRecordCreateTimestampMs: 0,
	}
	if err = p.SetAccountRecord(ctx, uid, accountRecord); err != nil {
		return nil, false, err
	}
	if err = p.client.Set(ctx, RedisKeyAccountUID(account), strconv.FormatUint(uid, 10), 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "set account uid failed, account: %s uid: %d %v", account, uid, xruntime.Location())
	}
	return accountRecord, true, nil
}

// GetAccountRecordByAccount 通过 account 读取 uid 和 AccountRecord。
// 如果账号映射存在但 AccountRecord 缺失或关键字段不一致，直接返回错误。
func (p *Redis) GetAccountRecordByAccount(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	uid, found, err := p.GetAccountUID(ctx, account)
	if err != nil || !found {
		return nil, found, err
	}
	accountRecord, err := p.GetAccountRecord(ctx, uid)
	if errors.Is(err, redis.Nil) {
		return nil, true, errors.Errorf("account record missing, account: %s uid: %d %v", account, uid, xruntime.Location())
	}
	if err != nil {
		return nil, true, err
	}
	if accountRecord == nil {
		return nil, true, errors.Errorf("account record is nil, account: %s uid: %d %v", account, uid, xruntime.Location())
	}
	if accountRecord.GetUid() != uid {
		return nil, true, errors.Errorf("account record uid mismatch, account: %s uid: %d record_uid: %d %v", account, uid, accountRecord.GetUid(), xruntime.Location())
	}
	if accountRecord.GetAccount() != account {
		return nil, true, errors.Errorf("account record account mismatch, account: %s uid: %d record_account: %s %v", account, uid, accountRecord.GetAccount(), xruntime.Location())
	}
	if accountRecord.GetAccountCreateTimestampMs() == 0 {
		return nil, true, errors.Errorf("account record create time is empty, account: %s uid: %d %v", account, uid, xruntime.Location())
	}
	return accountRecord, true, nil
}

// GetAccountUID 读取 account 到 uid 的映射。
// found 为 true 但返回错误时，表示 Redis 中存在不可用的 uid 值。
func (p *Redis) GetAccountUID(ctx context.Context, account string) (uint64, bool, error) {
	value, err := p.client.Get(ctx, RedisKeyAccountUID(account)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, errors.WithMessagef(err, "get account uid failed, account: %s %v", account, xruntime.Location())
	}
	uid, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, true, errors.WithMessagef(err, "parse account uid failed, account: %s value: %s %v", account, value, xruntime.Location())
	}
	if uid == 0 {
		return 0, true, errors.Errorf("parse account uid failed, account: %s value: %s %v", account, value, xruntime.Location())
	}
	return uid, true, nil
}
