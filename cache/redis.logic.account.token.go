package main

import (
	"context"
	"server/proto/pb"
	"strconv"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// SetAccountVerifyToken 写入账号级一次性验证 token。
// 使用 SETNX 保证未消费的 token 不会被覆盖，返回 false 表示 key 已存在。
func (p *Redis) SetAccountVerifyToken(ctx context.Context, account string, token string, expire time.Duration) (bool, error) {
	key := RedisKeyAccountToken(account)
	return p.client.SetNX(ctx, key, token, expire).Result()
}

// UseAccountVerifyToken 验证并消费账号级 token。
// 消费成功后删除 token key，避免同一 token 被重复使用。
func (p *Redis) UseAccountVerifyToken(ctx context.Context, account string, token string) (bool, error) {
	key := RedisKeyAccountToken(account)
	result, err := p.client.Eval(ctx, useAccountVerifyTokenScript, []string{key}, token).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "use account verify token from redis failed, account: %s, token: %s %v", account, token, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

// EnsureAccount 确保 account 有唯一 uid 和 UserRecord。
// 返回值 created 表示本次调用是否新建了账号。
func (p *Redis) EnsureAccount(ctx context.Context, account string) (*pb.UserRecord, bool, error) {
	for {
		userRecord, found, err := p.GetAccountUserRecord(ctx, account)
		if err != nil || found {
			return userRecord, false, err
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

		userRecord, created, err := p.createAccountAfterLock(ctx, account)
		if _, unlockErr := p.client.Del(ctx, RedisKeyAccountLock(account)).Result(); unlockErr != nil && err == nil {
			err = errors.WithMessagef(unlockErr, "unlock account create failed, account: %s %v", account, xruntime.Location())
		}
		return userRecord, created, err
	}
}

// createAccountAfterLock 在持有账号创建锁后创建账号数据。
// 创建前会再次查询账号映射，避免等待锁期间其他请求已经完成创建。
func (p *Redis) createAccountAfterLock(ctx context.Context, account string) (*pb.UserRecord, bool, error) {
	userRecord, found, err := p.GetAccountUserRecord(ctx, account)
	if err != nil || found {
		return userRecord, false, err
	}

	if err = p.client.SetNX(ctx, RedisKeyUserUIDSequence(), GCfgCustomRedisUIDSequenceSeed, 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "init uid sequence failed, account: %s %v", account, xruntime.Location())
	}
	uid, err := p.client.Incr(ctx, RedisKeyUserUIDSequence()).Uint64()
	if err != nil {
		return nil, false, errors.WithMessagef(err, "incr uid sequence failed, account: %s %v", account, xruntime.Location())
	}

	now := time.Now().UnixMilli()
	userRecord = &pb.UserRecord{
		Uid:               uid,
		Account:           account,
		AccountCreateTime: now,
		UserCreateTime:    0,
	}
	if err = p.SetUserRecord(ctx, uid, userRecord); err != nil {
		return nil, false, err
	}
	if err = p.client.Set(ctx, RedisKeyAccountUID(account), strconv.FormatUint(uid, 10), 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "set account uid failed, account: %s uid: %d %v", account, uid, xruntime.Location())
	}
	return userRecord, true, nil
}

// GetAccountUserRecord 通过 account 读取 uid 和 UserRecord。
// 如果账号映射存在但 UserRecord 缺失或关键字段为空，会补建或补齐后写回。
func (p *Redis) GetAccountUserRecord(ctx context.Context, account string) (*pb.UserRecord, bool, error) {
	uid, found, err := p.GetAccountUID(ctx, account)
	if err != nil || !found {
		return nil, found, err
	}
	userRecord, err := p.GetUserRecord(ctx, uid)
	if errors.Is(err, redis.Nil) {
		userRecord = &pb.UserRecord{
			Uid:               uid,
			Account:           account,
			AccountCreateTime: time.Now().UnixMilli(),
			UserCreateTime:    0,
		}
		if err = p.SetUserRecord(ctx, uid, userRecord); err != nil {
			return nil, true, err
		}
		return userRecord, true, nil
	}
	if err != nil {
		return nil, true, err
	}
	changed := false
	if userRecord.GetUid() == 0 {
		userRecord.Uid = uid
		changed = true
	}
	if userRecord.GetAccount() == "" {
		userRecord.Account = account
		changed = true
	}
	if userRecord.GetAccountCreateTime() == 0 {
		userRecord.AccountCreateTime = time.Now().UnixMilli()
		changed = true
	}
	if changed {
		if err = p.SetUserRecord(ctx, uid, userRecord); err != nil {
			return nil, true, err
		}
	}
	return userRecord, true, nil
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

// useAccountVerifyTokenScript 原子校验并消费 token。
// 成功时删除 token key；key 不存在或 token 不匹配时返回 0。
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
