package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func unaryCacheSetAccountRecord(uid uint64, accountRecord *pb.AccountRecord) error {
	_, err := pb.GXCacheServiceService.CacheSetAccountRecord(context.Background(), &pb.CacheSetAccountRecordReq{
		Uid:           uid,
		AccountRecord: accountRecord,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheSetAccountRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetAccountRecord uid:%d", uid)
	}
	return nil
}

func unaryCacheGetAccountRecord(uid uint64) (*pb.AccountRecord, error) {
	res, err := pb.GXCacheServiceService.CacheGetAccountRecord(context.Background(), &pb.CacheGetAccountRecordReq{
		Uid: uid,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheGetAccountRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetAccountRecord uid:%d", uid)
	}
	return res.GetAccountRecord(), nil
}
