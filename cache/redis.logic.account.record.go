package main

import (
	"context"
	"server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func (p *Redis) SetAccountRecord(ctx context.Context, aid uint64, record *pb.AccountRecord) error {
	data, err := proto.Marshal(record)
	if err != nil {
		return errors.WithMessagef(err, "marshal account record failed, aid: %d %v", aid, xruntime.Location())
	}
	key := RedisKeyAccountRecord(aid)
	if err := p.client.Set(ctx, key, data, 0).Err(); err != nil {
		return errors.WithMessagef(err, "set account record to redis failed, aid: %d %v", aid, xruntime.Location())
	}
	return nil
}

func (p *Redis) GetAccountRecord(ctx context.Context, aid uint64) (record *pb.AccountRecord, err error) {
	str, err := p.Get(ctx, RedisKeyAccountRecord(aid))
	if err != nil {
		return nil, errors.WithMessagef(err, "get account record from redis failed, aid: %d %v", aid, xruntime.Location())
	}
	record = &pb.AccountRecord{}
	if err := proto.Unmarshal([]byte(str), record); err != nil {
		return nil, errors.WithMessagef(err, "unmarshal account record failed, aid: %d %v", aid, xruntime.Location())
	}
	return record, nil
}
