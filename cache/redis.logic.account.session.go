package main

import (
	"context"
	"strconv"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

const beginAccountSessionScript = `
local expectedAccountSession = ARGV[1]
local index = 2
if expectedAccountSession == "" then
	if redis.call("EXISTS", KEYS[1]) == 1 then
		return 0
	end
else
	local current = redis.call("HGET", KEYS[1], "accountSession")
	if current == false or current ~= expectedAccountSession then
		return 0
	end
end
redis.call("DEL", KEYS[1])
local recordCount = tonumber(ARGV[index])
index = index + 1
for i = 1, recordCount do
	redis.call("HSET", KEYS[1], ARGV[index], ARGV[index + 1])
	index = index + 2
end
local expireSecond = tonumber(ARGV[index])
redis.call("EXPIRE", KEYS[1], expireSecond)
return 1
`

/*
expectedAccountSession == ""
  -> 期望当前 Redis 里没有 account:{aid}:session
  -> 如果 key 已存在, 返回 0, 不写入
  -> 如果 key 不存在, 写入新 session, 设置 TTL, 返回 1

expectedAccountSession != ""
  -> 期望当前 Redis hash 里的 accountSession 字段等于 expectedAccountSession
  -> 如果不存在或不匹配, 返回 0, 不写入
  -> 如果匹配, 删除旧 hash, 写入新 records, 设置 TTL, 返回 1
*/

func (p *Redis) BeginAccountSessionCAS(ctx context.Context, aid uint64, expectedAccountSession string, records map[string]string, expireSecond uint64) (bool, error) {
	key := RedisKeyAccountSession(aid)
	args := []any{expectedAccountSession}
	args = append(args, strconv.Itoa(len(records)))
	for field, value := range records {
		args = append(args, field, value)
	}
	args = append(args, strconv.FormatUint(expireSecond, 10))
	result, err := p.client.Eval(ctx, beginAccountSessionScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "begin account session in redis failed, aid: %d, expectedAccountSession: %s, records: %v %v", aid, expectedAccountSession, records, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

const endAccountSessionScript = `
local expectedAccountSession = ARGV[1]
local current = redis.call("HGET", KEYS[1], "accountSession")
if current == false or current ~= expectedAccountSession then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`

func (p *Redis) EndAccountSessionCAS(ctx context.Context, aid uint64, expectedAccountSession string) (bool, error) {
	key := RedisKeyAccountSession(aid)
	if expectedAccountSession == "" {
		return false, errors.Errorf("end account session expected is empty, aid: %d %v", aid, xruntime.Location())
	}
	result, err := p.client.Eval(ctx, endAccountSessionScript, []string{key}, expectedAccountSession).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "end account session in redis failed, aid: %d, expectedAccountSession: %s %v", aid, expectedAccountSession, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

const refreshAccountSessionScript = `
local expectedAccountSession = ARGV[1]
local current = redis.call("HGET", KEYS[1], "accountSession")
if current == false or current ~= expectedAccountSession then
	return 0
end
local expireSecond = tonumber(ARGV[2])
return redis.call("EXPIRE", KEYS[1], expireSecond)
`

func (p *Redis) RefreshAccountSessionCAS(ctx context.Context, aid uint64, expectedAccountSession string, expireSecond uint64) (bool, error) {
	key := RedisKeyAccountSession(aid)
	if expectedAccountSession == "" {
		return false, errors.Errorf("refresh account session expected is empty, aid: %d %v", aid, xruntime.Location())
	}
	args := []any{expectedAccountSession, strconv.FormatUint(expireSecond, 10)}
	result, err := p.client.Eval(ctx, refreshAccountSessionScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "refresh account session in redis failed, aid: %d, expectedAccountSession: %s %v", aid, expectedAccountSession, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

func (p *Redis) GetAccountSession(ctx context.Context, aid uint64) (map[string]string, error) {
	key := RedisKeyAccountSession(aid)
	fields := []string{
		accountSessionFieldGatewayKey,
		accountSessionFieldAccountSession,
		accountSessionFieldLoginTimestampMs,
		accountSessionFieldOnlineKey,
	}
	values, err := p.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, errors.WithMessagef(err, "get account session from redis failed, aid: %d %v", aid, xruntime.Location())
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
