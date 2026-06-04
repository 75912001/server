package main

import (
	"context"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"

	"github.com/redis/go-redis/v9"
)

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
	if userRecord.GetUid() != uid {
		return &pb.CacheSetUserRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "uid mismatch")
	}

	if err := GRedis.SetUserRecord(ctx, uid, userRecord); err != nil {
		return &pb.CacheSetUserRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetUserRecordRes{}, nil
}
