package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func unaryCacheSetAccountRecord(aid uint64, accountRecord *pb.AccountRecord) error {
	_, err := pb.GXCacheServiceService.CacheSetAccountRecord(context.Background(), &pb.CacheSetAccountRecordReq{
		Aid:           aid,
		AccountRecord: accountRecord,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d, code:%v, message:%s", aid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d", aid)
	}
	return nil
}

func unaryCacheGetAccountRecord(aid uint64) (*pb.AccountRecord, error) {
	res, err := pb.GXCacheServiceService.CacheGetAccountRecord(context.Background(), &pb.CacheGetAccountRecordReq{
		Aid: aid,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheGetAccountRecord aid:%d, code:%v, message:%s", aid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetAccountRecord aid:%d", aid)
	}
	return res.GetAccountRecord(), nil
}
