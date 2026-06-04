package main

import (
	"context"
	"strconv"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// SetUserSessionRecord 批量写入用户在线 session hash 字段。
// 该接口只负责 HSET 字段，不负责设置 TTL；TTL 由替换或续期接口维护。
func (p *Redis) SetUserSessionRecord(ctx context.Context, uid uint64, records map[string]string) error {
	key := RedisKeyUserSession(uid)
	if err := p.client.HSet(ctx, key, records).Err(); err != nil {
		return errors.WithMessagef(err, "set user session record to redis failed, uid: %d, records: %v %v", uid, records, xruntime.Location())
	}
	return nil
}

// replaceUserSessionRecordScript 在 Redis 侧原子完成 expected 校验、字段替换和 TTL 刷新。
// 不存在的 hash 字段按空字符串处理，用于支持首次登录时的空在线态 CAS。
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

// expireUserSessionRecordScript 在 expected 完全匹配时刷新 session TTL。
// 返回值沿用 Redis EXPIRE 语义：1 表示成功刷新，0 表示 key 不存在或校验失败。
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

// delUserSessionRecordScript 在 expected 完全匹配时删除用户在线 session。
// 用脚本合并 HGET 校验和 DEL，避免旧 online 的迟到删除误删新 session。
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

// ReplaceUserSessionRecord 原子替换用户在线 session 字段，并按需刷新 TTL。
// expected 用于确认当前在线态仍属于调用方，records 必须由上层保证字段完整。
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

// DelUserSessionRecord 在 expected 匹配时删除用户在线 session。
// expected 不能为空，避免调用方绕过 session identity 校验直接删除在线态。
func (p *Redis) DelUserSessionRecord(ctx context.Context, uid uint64, expected map[string]string) (bool, error) {
	key := RedisKeyUserSession(uid)
	if len(expected) == 0 {
		return false, errors.Errorf("delete user session record expected is empty, uid: %d %v", uid, xruntime.Location())
	}
	args := make([]any, 0, 1+len(expected)*2)
	args = append(args, strconv.Itoa(len(expected)))
	for field, value := range expected {
		args = append(args, field, value)
	}
	result, err := p.client.Eval(ctx, delUserSessionRecordScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "delete user session record from redis failed, uid: %d, expected: %v %v", uid, expected, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

// SetUserSessionExpire 在 expected 匹配时刷新用户在线 session TTL。
// expected 不能为空，续期必须绑定稳定 identity，避免旧 session 续活新在线态。
func (p *Redis) SetUserSessionExpire(ctx context.Context, uid uint64, expire time.Duration, expected map[string]string) (bool, error) {
	key := RedisKeyUserSession(uid)
	if len(expected) == 0 {
		return false, errors.Errorf("set user session expire expected is empty, uid: %d %v", uid, xruntime.Location())
	}
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

// GetUserSessionRecord 按字段批量读取用户在线 session。
// Redis 中不存在的字段会以 nil 返回，这里只保留实际存在的字段。
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
