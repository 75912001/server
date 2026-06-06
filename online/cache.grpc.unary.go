package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func unaryCacheSetUserRecord(uid uint64, userRecord *pb.UserRecord) error {
	_, err := pb.GXCacheServiceService.CacheSetUserRecord(context.Background(), &pb.CacheSetUserRecordReq{
		Uid:        uid,
		UserRecord: userRecord,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheSetUserRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetUserRecord uid:%d", uid)
	}
	return nil
}

func unaryCacheGetUserRecord(uid uint64) (*pb.UserRecord, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserRecord(context.Background(), &pb.CacheGetUserRecordReq{
		Uid: uid,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheGetUserRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetUserRecord uid:%d", uid)
	}
	return res.GetUserRecord(), nil
}
