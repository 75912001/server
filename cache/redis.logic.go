package main

import (
	"context"
	"time"
)

// VerifyUserToken 验证用户令牌

// SetVerifyUserToken 设置用户令牌
func (r *Redis) SetVerifyUserToken(ctx context.Context, uid uint64, token string, expire time.Duration) (bool, error) {
	key := RedisKeyUserToken(uid, token)
	return r.client.SetNX(ctx, key, "1", expire).Result()
}

// VerifyUserToken 验证用户令牌
func (r *Redis) VerifyUserToken(ctx context.Context, uid uint64, token string) (bool, error) {
	key := RedisKeyUserToken(uid, token)
	res, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}
