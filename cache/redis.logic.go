package main

import (
	"context"
	"server/proto/pb"
	"strconv"
	"strings"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

func normalizeAccount(account string) string {
	return strings.TrimSpace(account)
}

func (p *Redis) SetAccountVerifyToken(ctx context.Context, account string, token string, expire time.Duration) (bool, error) {
	key := RedisKeyAccountToken(account)
	return p.client.SetNX(ctx, key, token, expire).Result()
}

func (p *Redis) UseAccountVerifyToken(ctx context.Context, account string, token string) (bool, error) {
	key := RedisKeyAccountToken(account)
	usedValue := accountVerifyTokenUsedValue(token)
	result, err := p.client.Eval(ctx, useAccountVerifyTokenScript, []string{key}, token, usedValue).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "use account verify token from redis failed, account: %s, token: %s %v", account, token, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

func (p *Redis) EnsureAccount(ctx context.Context, account string) (*pb.UserRecord, bool, error) {
	account = normalizeAccount(account)
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

func (p *Redis) GetAccountUserRecord(ctx context.Context, account string) (*pb.UserRecord, bool, error) {
	account = normalizeAccount(account)
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

func (p *Redis) GetAccountUID(ctx context.Context, account string) (uint64, bool, error) {
	account = normalizeAccount(account)
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

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

const useAccountVerifyTokenScript = `
local current = redis.call("GET", KEYS[1])
if current == false then
	return 0
end
if current ~= ARGV[1] then
	return 0
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl > 0 then
	redis.call("SET", KEYS[1], ARGV[2], "PX", ttl)
elseif ttl == -1 then
	redis.call("SET", KEYS[1], ARGV[2])
else
	return 0
end
return 1
`

func accountVerifyTokenUsedValue(token string) string {
	return "used:" + token
}

// UserRecord 用户记录

func (p *Redis) SetUserRecord(ctx context.Context, uid uint64, record *pb.UserRecord) error {
	data, err := proto.Marshal(record)
	if err != nil {
		return errors.WithMessagef(err, "marshal user record failed, uid: %d %v", uid, xruntime.Location())
	}
	key := RedisKeyUserRecord(uid)
	if err := p.client.Set(ctx, key, data, 0).Err(); err != nil {
		return errors.WithMessagef(err, "set user record to redis failed, uid: %d %v", uid, xruntime.Location())
	}
	return nil
}

func (p *Redis) GetUserRecord(ctx context.Context, uid uint64) (record *pb.UserRecord, err error) {
	str, err := p.Get(ctx, RedisKeyUserRecord(uid))
	if err != nil {
		return nil, errors.WithMessagef(err, "get user record from redis failed, uid: %d %v", uid, xruntime.Location())
	}
	record = &pb.UserRecord{}
	if err := proto.Unmarshal([]byte(str), record); err != nil {
		return nil, errors.WithMessagef(err, "unmarshal user record failed, uid: %d %v", uid, xruntime.Location())
	}
	return record, nil
}

// 用户 Session 记录

func (p *Redis) SetUserSessionRecord(ctx context.Context, uid uint64, records map[string]string) error {
	key := RedisKeyUserSession(uid)
	if err := p.client.HSet(ctx, key, records).Err(); err != nil {
		return errors.WithMessagef(err, "set user session record to redis failed, uid: %d, records: %v %v", uid, records, xruntime.Location())
	}
	return nil
}

const replaceUserSessionRecordScript = `
local expectedCount = tonumber(ARGV[1])
local index = 2
for i = 1, expectedCount do
	local field = ARGV[index]
	local expected = ARGV[index + 1]
	index = index + 2
	local current = redis.call("HGET", KEYS[1], field)
	if current == false then
		current = ""
	end
	if current ~= expected then
		return 0
	end
end
local recordCount = tonumber(ARGV[index])
index = index + 1
for i = 1, recordCount do
	redis.call("HSET", KEYS[1], ARGV[index], ARGV[index + 1])
	index = index + 2
end
local expireSecond = tonumber(ARGV[index])
if expireSecond > 0 then
	redis.call("EXPIRE", KEYS[1], expireSecond)
end
return 1
`

const expireUserSessionRecordScript = `
local expectedCount = tonumber(ARGV[1])
local index = 2
for i = 1, expectedCount do
	local field = ARGV[index]
	local expected = ARGV[index + 1]
	index = index + 2
	local current = redis.call("HGET", KEYS[1], field)
	if current == false then
		current = ""
	end
	if current ~= expected then
		return 0
	end
end
local expireSecond = tonumber(ARGV[index])
return redis.call("EXPIRE", KEYS[1], expireSecond)
`

const delUserSessionRecordScript = `
local expectedCount = tonumber(ARGV[1])
local index = 2
for i = 1, expectedCount do
	local field = ARGV[index]
	local expected = ARGV[index + 1]
	index = index + 2
	local current = redis.call("HGET", KEYS[1], field)
	if current == false then
		current = ""
	end
	if current ~= expected then
		return 0
	end
end
redis.call("DEL", KEYS[1])
return 1
`

func (p *Redis) ReplaceUserSessionRecord(ctx context.Context, uid uint64, expected map[string]string, records map[string]string, expireSecond uint64) (bool, error) {
	key := RedisKeyUserSession(uid)
	args := make([]any, 0, 3+len(expected)*2+len(records)*2)
	args = append(args, strconv.Itoa(len(expected)))
	for field, value := range expected {
		args = append(args, field, value)
	}
	args = append(args, strconv.Itoa(len(records)))
	for field, value := range records {
		args = append(args, field, value)
	}
	args = append(args, strconv.FormatUint(expireSecond, 10))
	result, err := p.client.Eval(ctx, replaceUserSessionRecordScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "replace user session record in redis failed, uid: %d, expected: %v, records: %v %v", uid, expected, records, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

func (p *Redis) DelUserSessionRecord(ctx context.Context, uid uint64, expected map[string]string) error {
	key := RedisKeyUserSession(uid)
	if len(expected) == 0 {
		return errors.Errorf("delete user session record expected is empty, uid: %d %v", uid, xruntime.Location())
	}
	args := make([]any, 0, 1+len(expected)*2)
	args = append(args, strconv.Itoa(len(expected)))
	for field, value := range expected {
		args = append(args, field, value)
	}
	if _, err := p.client.Eval(ctx, delUserSessionRecordScript, []string{key}, args...).Result(); err != nil {
		return errors.WithMessagef(err, "delete user session record from redis failed, uid: %d, expected: %v %v", uid, expected, xruntime.Location())
	}
	return nil
}

func redisScriptResultIsOK(result any) bool {
	switch v := result.(type) {
	case int64:
		return v == 1
	case int:
		return v == 1
	case string:
		return v == "1"
	default:
		return false
	}
}

func (p *Redis) SetUserSessionExpire(ctx context.Context, uid uint64, expire time.Duration, expected map[string]string) (bool, error) {
	key := RedisKeyUserSession(uid)
	if len(expected) != 0 {
		args := make([]any, 0, 2+len(expected)*2)
		args = append(args, strconv.Itoa(len(expected)))
		for field, value := range expected {
			args = append(args, field, value)
		}
		args = append(args, strconv.FormatInt(int64(expire/time.Second), 10))
		result, err := p.client.Eval(ctx, expireUserSessionRecordScript, []string{key}, args...).Result()
		if err != nil {
			return false, errors.WithMessagef(err, "set user session expire to redis failed, uid: %d, expected: %v %v", uid, expected, xruntime.Location())
		}
		return redisScriptResultIsOK(result), nil
	}
	ok, err := p.client.Expire(ctx, key, expire).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "set user session expire to redis failed, uid: %d %v", uid, xruntime.Location())
	}
	return ok, nil
}

func (p *Redis) GetUserSessionRecord(ctx context.Context, uid uint64, fields []string) (map[string]string, error) {
	key := RedisKeyUserSession(uid)
	values, err := p.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, errors.WithMessagef(err, "get user session record from redis failed, uid: %d, fields: %v %v", uid, fields, xruntime.Location())
	}
	records := make(map[string]string, len(values))
	for i, value := range values {
		if value == nil {
			continue
		}
		records[fields[i]] = value.(string)
	}
	return records, nil
}
