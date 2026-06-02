package main

import (
	"context"
	"server/proto/pb"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// VerifyUserToken 验证用户令牌

// SetVerifyUserToken 设置用户令牌
func (p *Redis) SetVerifyUserToken(ctx context.Context, uid uint64, token string, expire time.Duration) (bool, error) {
	key := RedisKeyUserToken(uid, token)
	return p.client.SetNX(ctx, key, "1", expire).Result()
}

// VerifyUserToken 验证用户令牌
func (p *Redis) VerifyUserToken(ctx context.Context, uid uint64, token string) (bool, error) {
	key := RedisKeyUserToken(uid, token)
	res, err := p.client.Del(ctx, key).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "delete user token from redis failed, uid: %d, token: %s %v", uid, token, xruntime.Location())
	}
	return res == 1, nil
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

func (p *Redis) DelUserSessionRecord(ctx context.Context, uid uint64) error {
	key := RedisKeyUserSession(uid)
	if err := p.client.Del(ctx, key).Err(); err != nil {
		return errors.WithMessagef(err, "delete user session record from redis failed, uid: %d %v", uid, xruntime.Location())
	}
	return nil
}

func (p *Redis) SetUserSessionExpire(ctx context.Context, uid uint64, expire time.Duration) (bool, error) {
	key := RedisKeyUserSession(uid)
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
