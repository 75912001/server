package main

import (
	"context"
	"time"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"

	"github.com/redis/go-redis/v9"
)

// todo menglc 当前直接从redis获取数据,后续可以考虑加一层本地缓存,比如sync.Map,减少redis访问压力
func (s *cacheGRPCServer) CacheGetUserRecord(ctx context.Context, req *pb.CacheGetUserRecordReq) (*pb.CacheGetUserRecordRes, error) {
	uid := req.GetUid()
	if uid == 0 {
		return &pb.CacheGetUserRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid uid:0")
	}

	userRecord, err := GRedis.GetUserRecord(ctx, uid)
	if errors.Is(err, redis.Nil) {
		return &pb.CacheGetUserRecordRes{}, grpcstatus.Error(grpccodes.NotFound, err.Error())
	}
	if err != nil {
		return &pb.CacheGetUserRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}

	return &pb.CacheGetUserRecordRes{
		UserRecord: userRecord,
	}, nil
}

func (s *cacheGRPCServer) CacheSetUserRecord(ctx context.Context, req *pb.CacheSetUserRecordReq) (*pb.CacheSetUserRecordRes, error) {
	uid := req.GetUid()
	userRecord := req.GetUserRecord()
	if uid == 0 || userRecord == nil {
		return &pb.CacheSetUserRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	if userRecord.GetUid() != 0 && userRecord.GetUid() != uid {
		return &pb.CacheSetUserRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "uid mismatch")
	}

	if err := GRedis.SetUserRecord(ctx, uid, userRecord); err != nil {
		return &pb.CacheSetUserRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetUserRecordRes{}, nil
}

func (s *cacheGRPCServer) CacheSetVerifyUserToken(ctx context.Context, req *pb.CacheSetVerifyUserTokenReq) (*pb.CacheSetVerifyUserTokenRes, error) {
	uid := req.GetUid()
	token := req.GetToken()
	expireSecond := req.GetExpireSecond()
	if uid == 0 || token == "" || expireSecond == 0 {
		return &pb.CacheSetVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	ok, err := GRedis.SetVerifyUserToken(ctx, uid, token, time.Duration(expireSecond)*time.Second)
	if err != nil {
		return &pb.CacheSetVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		return &pb.CacheSetVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.AlreadyExists, "token already exists")
	}
	return &pb.CacheSetVerifyUserTokenRes{}, nil
}

func (s *cacheGRPCServer) CacheVerifyUserToken(ctx context.Context, req *pb.CacheVerifyUserTokenReq) (*pb.CacheVerifyUserTokenRes, error) {
	uid := req.GetUid()
	token := req.GetToken()
	if uid == 0 || token == "" {
		return &pb.CacheVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	ok, err := GRedis.VerifyUserToken(ctx, uid, token)
	if err != nil {
		return &pb.CacheVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		return &pb.CacheVerifyUserTokenRes{}, grpcstatus.Error(grpccodes.NotFound, "token not found or expired")
	}

	return &pb.CacheVerifyUserTokenRes{}, nil
}
