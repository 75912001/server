package main

import (
	"context"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"
)

func (s *cacheGRPCServer) CacheSetAccountVerifyToken(ctx context.Context, req *pb.CacheSetAccountVerifyTokenReq) (*pb.CacheSetAccountVerifyTokenRes, error) {
	account := req.GetAccount()
	token := req.GetToken()
	expireSecond := req.GetExpireSecond()
	if account == "" || token == "" || expireSecond == 0 {
		return &pb.CacheSetAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	ok, err := GRedis.SetAccountVerifyToken(ctx, account, token, time.Duration(expireSecond)*time.Second)
	if err != nil {
		return &pb.CacheSetAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		return &pb.CacheSetAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.AlreadyExists, "token already exists")
	}
	return &pb.CacheSetAccountVerifyTokenRes{}, nil
}

func (s *cacheGRPCServer) CacheUseAccountVerifyToken(ctx context.Context, req *pb.CacheUseAccountVerifyTokenReq) (*pb.CacheUseAccountVerifyTokenRes, error) {
	account := req.GetAccount()
	token := req.GetToken()
	if account == "" || token == "" {
		return &pb.CacheUseAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid param")
	}
	ok, err := GRedis.UseAccountVerifyToken(ctx, account, token)
	if err != nil {
		return &pb.CacheUseAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		return &pb.CacheUseAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.NotFound, "token not found or used")
	}
	userRecord, _, err := GRedis.EnsureAccount(ctx, account)
	if err != nil {
		return &pb.CacheUseAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if userRecord == nil || userRecord.GetUid() == 0 {
		return &pb.CacheUseAccountVerifyTokenRes{}, grpcstatus.Error(grpccodes.Internal, "account uid is empty")
	}
	return &pb.CacheUseAccountVerifyTokenRes{
		Uid: userRecord.GetUid(),
	}, nil
}
