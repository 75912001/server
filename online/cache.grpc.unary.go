package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func unaryCacheVerifyUserToken(uid uint64, token string) (*pb.CacheVerifyUserTokenRes, error) {
	res, err := pb.GXCacheServiceService.CacheVerifyUserToken(context.Background(), &pb.CacheVerifyUserTokenReq{
		Uid:   uid,
		Token: token,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return nil, errors.WithMessagef(err, "CacheVerifyUserToken uid:%d token:%s, code:%v, message:%s", uid, token, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheVerifyUserToken uid:%d token:%s", uid, token)
	}
	return res, nil
}

func unaryCacheGetUserRecord(uid uint64) (*pb.CacheGetUserRecordRes, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserRecord(context.Background(), &pb.CacheGetUserRecordReq{
		Uid: uid,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return nil, errors.WithMessagef(err, "CacheGetUserRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetUserRecord uid:%d", uid)
	}
	return res, nil
}
