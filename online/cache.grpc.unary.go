package main

import (
	"context"

	pb "server/proto/pb"
)

func unaryCacheVerifyUserToken(uid uint64, token string) (*pb.CacheVerifyUserTokenRes, error) {
	res, err := pb.GXCacheServiceService.CacheVerifyUserToken(context.Background(), &pb.CacheVerifyUserTokenReq{
		Uid:   uid,
		Token: token,
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func unaryCacheGetUserRecord(uid uint64) (*pb.CacheGetUserRecordRes, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserRecord(context.Background(), &pb.CacheGetUserRecordReq{
		Uid: uid,
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
