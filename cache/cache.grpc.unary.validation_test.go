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

	_, err := s.CacheSetUserSessionRecord(ctx, &pb.CacheSetUserSessionRecordReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetUserSessionRecord(ctx, &pb.CacheSetUserSessionRecordReq{
		Uid:     1,
		Records: []*pb.CacheUserSessionRecord{{Field: pb.CacheUserSessionField_CacheUserSessionField_Unspecified}},
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheGetUserSessionRecord(ctx, &pb.CacheGetUserSessionRecordReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheGetUserSessionRecord(ctx, &pb.CacheGetUserSessionRecordReq{
		Uid:    1,
		Fields: []pb.CacheUserSessionField{pb.CacheUserSessionField_CacheUserSessionField_Unspecified},
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheReplaceUserSessionRecord(ctx, &pb.CacheReplaceUserSessionRecordReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheSetUserSessionExpire(ctx, &pb.CacheSetUserSessionExpireReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = s.CacheDelUserSessionRecord(ctx, &pb.CacheDelUserSessionRecordReq{Uid: 1})
	requireStatusCode(t, err, grpccodes.InvalidArgument)
}
