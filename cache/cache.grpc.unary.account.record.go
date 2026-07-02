package main

import (
	"context"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"

	"github.com/redis/go-redis/v9"
)

func (s *cacheGRPCServer) CacheGetAccountRecord(ctx context.Context, req *pb.CacheGetAccountRecordReq) (*pb.CacheGetAccountRecordRes, error) {
	aid := req.GetAid()
	if aid == 0 {
		return &pb.CacheGetAccountRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid aid:0")
	}

	accountRecord, err := GRedis.GetAccountRecord(ctx, aid)
	if errors.Is(err, redis.Nil) {
		return &pb.CacheGetAccountRecordRes{}, grpcstatus.Error(grpccodes.NotFound, err.Error())
	}
	if err != nil {
		return &pb.CacheGetAccountRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}

	return &pb.CacheGetAccountRecordRes{
		AccountRecord: accountRecord,
	}, nil
}

func (s *cacheGRPCServer) CacheSetAccountRecord(ctx context.Context, req *pb.CacheSetAccountRecordReq) (*pb.CacheSetAccountRecordRes, error) {
	aid := req.GetAid()
	accountRecord := req.GetAccountRecord()
	if aid == 0 || accountRecord == nil {
		return &pb.CacheSetAccountRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	if accountRecord.GetAid() != aid {
		return &pb.CacheSetAccountRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "aid mismatch")
	}

	if err := GRedis.SetAccountRecord(ctx, aid, accountRecord); err != nil {
		return &pb.CacheSetAccountRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetAccountRecordRes{}, nil
}
