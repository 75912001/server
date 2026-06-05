package main

import (
	"context"
	"strconv"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

const beginUserSessionScript = `
local expectedUserSession = ARGV[1]
local index = 2
if expectedUserSession == "" then
	if redis.call("EXISTS", KEYS[1]) == 1 then
		return 0
	end
else
	local current = redis.call("HGET", KEYS[1], "userSession")
	if current == false or current ~= expectedUserSession then
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

func (p *Redis) BeginUserSessionCAS(ctx context.Context, uid uint64, expectedUserSession string, records map[string]string, expireSecond uint64) (bool, error) {
	key := RedisKeyUserSession(uid)
	args := []any{expectedUserSession}
	args = append(args, strconv.Itoa(len(records)))
	for field, value := range records {
		args = append(args, field, value)
	}
	args = append(args, strconv.FormatUint(expireSecond, 10))
	result, err := p.client.Eval(ctx, beginUserSessionScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "begin user session in redis failed, uid: %d, expectedUserSession: %s, records: %v %v", uid, expectedUserSession, records, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

const endUserSessionScript = `
local expectedUserSession = ARGV[1]
local current = redis.call("HGET", KEYS[1], "userSession")
if current == false or current ~= expectedUserSession then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`

func (p *Redis) EndUserSessionCAS(ctx context.Context, uid uint64, expectedUserSession string) (bool, error) {
	key := RedisKeyUserSession(uid)
	if expectedUserSession == "" {
		return false, errors.Errorf("end user session expected is empty, uid: %d %v", uid, xruntime.Location())
	}
	result, err := p.client.Eval(ctx, endUserSessionScript, []string{key}, expectedUserSession).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "end user session in redis failed, uid: %d, expectedUserSession: %s %v", uid, expectedUserSession, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

const refreshUserSessionScript = `
local expectedUserSession = ARGV[1]
local current = redis.call("HGET", KEYS[1], "userSession")
if current == false or current ~= expectedUserSession then
	return 0
end
local expireSecond = tonumber(ARGV[2])
return redis.call("EXPIRE", KEYS[1], expireSecond)
`

func (p *Redis) RefreshUserSessionCAS(ctx context.Context, uid uint64, expectedUserSession string, expireSecond uint64) (bool, error) {
	key := RedisKeyUserSession(uid)
	if expectedUserSession == "" {
		return false, errors.Errorf("refresh user session expected is empty, uid: %d %v", uid, xruntime.Location())
	}
	args := []any{expectedUserSession, strconv.FormatUint(expireSecond, 10)}
	result, err := p.client.Eval(ctx, refreshUserSessionScript, []string{key}, args...).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "refresh user session in redis failed, uid: %d, expectedUserSession: %s %v", uid, expectedUserSession, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

func (p *Redis) GetUserSession(ctx context.Context, uid uint64) (map[string]string, error) {
	key := RedisKeyUserSession(uid)
	fields := []string{
		userSessionFieldGatewayKey,
		userSessionFieldUserSession,
		userSessionFieldLoginTime,
		userSessionFieldOnlineKey,
	}
	values, err := p.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, errors.WithMessagef(err, "get user session from redis failed, uid: %d %v", uid, xruntime.Location())
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
