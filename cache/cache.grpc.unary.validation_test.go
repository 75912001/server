package main

import (
	"context"
	"testing"

	grpccodes "google.golang.org/grpc/codes"

	pb "server/proto/pb"
)

func TestAccountVerifyTokenHandlerInvalidArguments(t *testing.T) {
	s := &cacheGRPCServer{}
	ctx := context.Background()

	_, err := s.CacheSetAccountVerifyToken(ctx, &pb.CacheSetAccountVerifyTokenReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheUseAccountVerifyToken(ctx, &pb.CacheUseAccountVerifyTokenReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)
}

func TestAccountRecordHandlerInvalidArguments(t *testing.T) {
	s := &cacheGRPCServer{}
	ctx := context.Background()

	_, err := s.CacheGetAccountRecord(ctx, &pb.CacheGetAccountRecordReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetAccountRecord(ctx, &pb.CacheSetAccountRecordReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetAccountRecord(ctx, &pb.CacheSetAccountRecordReq{
		Uid:           1,
		AccountRecord: &pb.AccountRecord{Uid: 2},
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)
}

func TestUserSessionHandlerInvalidArguments(t *testing.T) {
	s := &cacheGRPCServer{}
	ctx := context.Background()

	_, err := s.CacheGetUserSession(ctx, &pb.CacheGetUserSessionReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheBeginUserSessionCAS(ctx, &pb.CacheBeginUserSessionCASReq{
		Uid:          1,
		ExpireSecond: 1,
		GatewayKey:   "gateway-1",
		UserSession:  "session-1",
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheBeginUserSessionCAS(ctx, &pb.CacheBeginUserSessionCASReq{
		Uid:              1,
		ExpireSecond:     1,
		GatewayKey:       "gateway-1",
		UserSession:      "session-1",
		LoginTimestampMs: 123,
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheEndUserSessionCAS(ctx, &pb.CacheEndUserSessionCASReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheRefreshUserSessionCAS(ctx, &pb.CacheRefreshUserSessionCASReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheRefreshUserSessionCAS(ctx, &pb.CacheRefreshUserSessionCASReq{
		Uid:          1,
		ExpireSecond: 1,
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)
}
