package main

import (
	"context"
	"server/proto/pb"
	"strconv"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// VerifyUserToken 验证用户令牌

// SetVerifyUserToken 设置用户令牌
func (p *Redis) SetVerifyUserToken(ctx context.Context, uid uint64, token string, expire time.Duration) (bool, error) {
	key := RedisKeyUserToken(uid)
	return p.client.SetNX(ctx, key, token, expire).Result()
}

const verifyUserTokenScript = `
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

// VerifyUserToken 验证用户令牌
func (p *Redis) VerifyUserToken(ctx context.Context, uid uint64, token string) (bool, error) {
	key := RedisKeyUserToken(uid)
	result, err := p.client.Eval(ctx, verifyUserTokenScript, []string{key}, token).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "verify user token from redis failed, uid: %d, token: %s %v", uid, token, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
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
