package main

import (
	"context"
	"server/common"
	"time"

	xerror "github.com/75912001/xlib/error"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"

	pb "server/proto/pb"

	"github.com/redis/go-redis/v9"
)

// todo menglc 当前直接从redis获取数据,后续可以考虑加一层本地缓存,比如sync.Map,减少redis访问压力
func (s *cacheGRPCServer) CacheGetUserRecord(ctx context.Context, req *pb.CacheGetUserRecordReq) (*pb.CacheGetUserRecordRes, error) {
	uid := req.GetUid()
	if uid == 0 {
		return &pb.CacheGetUserRecordRes{
			Code: common.ECCacheInvalidArgument.Code(),
			Msg:  common.ECCacheInvalidArgument.Desc(),
		}, nil
	}

	userRecord, err := GRedis.GetUserRecord(ctx, uid)
	if errors.Is(err, redis.Nil) {
		return &pb.CacheGetUserRecordRes{
			Code: common.ECCacheKeyNotFound.Code(),
			Msg:  common.ECCacheKeyNotFound.Desc(),
		}, nil
	}
	if err != nil {
		return &pb.CacheGetUserRecordRes{
			Code: common.ECCacheRedisError.Code(),
			Msg:  errors.WithMessagef(err, "%v %v", common.ECCacheRedisError.Desc(), xruntime.Location()).Error(),
		}, nil
	}

	return &pb.CacheGetUserRecordRes{
		Code:       xerror.Success.Code(),
		Msg:        xerror.Success.Desc(),
		UserRecord: userRecord,
	}, nil
}

func (s *cacheGRPCServer) CacheSetVerifyUserToken(ctx context.Context, req *pb.CacheSetVerifyUserTokenReq) (*pb.CacheSetVerifyUserTokenRes, error) {
	uid := req.GetUid()
	token := req.GetToken()
	expireSecond := req.GetExpireSecond()
	if uid == 0 || token == "" || expireSecond == 0 {
		return &pb.CacheSetVerifyUserTokenRes{
			Code: common.ECCacheInvalidArgument.Code(),
			Msg:  common.ECCacheInvalidArgument.Desc(),
			Ok:   false,
		}, nil
	}
	ok, err := GRedis.SetVerifyUserToken(ctx, uid, token, time.Duration(expireSecond)*time.Second)
	if err != nil {
		return &pb.CacheSetVerifyUserTokenRes{
			Code: common.ECCacheRedisError.Code(),
			Msg:  errors.WithMessagef(err, "%v %v", common.ECCacheRedisError.Desc(), xruntime.Location()).Error(),
			Ok:   false,
		}, nil
	}
	return &pb.CacheSetVerifyUserTokenRes{
		Code: xerror.Success.Code(),
		Msg:  xerror.Success.Desc(),
		Ok:   ok,
	}, nil
}

func (s *cacheGRPCServer) CacheVerifyUserToken(ctx context.Context, req *pb.CacheVerifyUserTokenReq) (*pb.CacheVerifyUserTokenRes, error) {
	uid := req.GetUid()
	token := req.GetToken()
	if uid == 0 || token == "" {
		return &pb.CacheVerifyUserTokenRes{
			Code: common.ECCacheInvalidArgument.Code(),
			Msg:  common.ECCacheInvalidArgument.Desc(),
		}, nil
	}
	ok, err := GRedis.VerifyUserToken(ctx, uid, token)
	if err != nil {
		return &pb.CacheVerifyUserTokenRes{
			Code: common.ECCacheRedisError.Code(),
			Msg:  errors.WithMessagef(err, "%v %v", common.ECCacheRedisError.Desc(), xruntime.Location()).Error(),
			Ok:   false,
		}, nil
	}
	if !ok {
		return &pb.CacheVerifyUserTokenRes{
			Code: common.ECCacheKeyNotFound.Code(),
			Msg:  common.ECCacheKeyNotFound.Desc(),
			Ok:   false,
		}, nil
	}

	return &pb.CacheVerifyUserTokenRes{
		Code: xerror.Success.Code(),
		Msg:  xerror.Success.Desc(),
		Ok:   true,
	}, nil
}
