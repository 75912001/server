package main

import (
	"context"
	"time"
)

// VerifyUserToken 验证用户令牌

// RedisSetVerifyUserToken 设置用户令牌
func (r *Redis) RedisSetVerifyUserToken(ctx context.Context, uid uint64, token string, expire time.Duration) (bool, error) {
	return r.client.SetNX(ctx, cfgUserTokenKey(uid, token), "1", expire).Result()
}

// RedisVerifyUserToken 验证用户令牌
func (r *Redis) RedisVerifyUserToken(ctx context.Context, uid uint64, token string) (bool, error) {
	key := cfgUserTokenKey(uid, token)
	res, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}
