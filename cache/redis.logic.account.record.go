package main

import (
	"context"
	"server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func (p *Redis) SetAccountRecord(ctx context.Context, uid uint64, record *pb.AccountRecord) error {
	data, err := proto.Marshal(record)
	if err != nil {
		return errors.WithMessagef(err, "marshal account record failed, uid: %d %v", uid, xruntime.Location())
	}
	key := RedisKeyAccountRecord(uid)
	if err := p.client.Set(ctx, key, data, 0).Err(); err != nil {
		return errors.WithMessagef(err, "set account record to redis failed, uid: %d %v", uid, xruntime.Location())
	}
	return nil
}

func (p *Redis) GetAccountRecord(ctx context.Context, uid uint64) (record *pb.AccountRecord, err error) {
	str, err := p.Get(ctx, RedisKeyAccountRecord(uid))
	if err != nil {
		return nil, errors.WithMessagef(err, "get account record from redis failed, uid: %d %v", uid, xruntime.Location())
	}
	record = &pb.AccountRecord{}
	if err := proto.Unmarshal([]byte(str), record); err != nil {
		return nil, errors.WithMessagef(err, "unmarshal account record failed, uid: %d %v", uid, xruntime.Location())
	}
	return record, nil
}
