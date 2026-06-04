package main

import (
	"context"
	"server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

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
