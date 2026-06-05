package main

import (
	"context"
	"testing"

	grpccodes "google.golang.org/grpc/codes"

	pb "server/proto/pb"
)

func TestAccountTokenHandlerInvalidArguments(t *testing.T) {
	s := &cacheGRPCServer{}
	ctx := context.Background()

	_, err := s.CacheSetAccountVerifyToken(ctx, &pb.CacheSetAccountVerifyTokenReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheUseAccountVerifyToken(ctx, &pb.CacheUseAccountVerifyTokenReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)
}

func TestUserRecordHandlerInvalidArguments(t *testing.T) {
	s := &cacheGRPCServer{}
	ctx := context.Background()

	_, err := s.CacheGetUserRecord(ctx, &pb.CacheGetUserRecordReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetUserRecord(ctx, &pb.CacheSetUserRecordReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetUserRecord(ctx, &pb.CacheSetUserRecordReq{
		Uid:        1,
		UserRecord: &pb.UserRecord{Uid: 2},
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
		Uid:          1,
		ExpireSecond: 1,
		GatewayKey:   "gateway-1",
		UserSession:  "session-1",
		LoginTimeMs:  123,
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
