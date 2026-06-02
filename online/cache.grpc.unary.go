package main

import (
	"context"
	"strconv"
	"time"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
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

type cacheUserSession struct {
	gatewayKey string
	onlineKey  string
}

func unaryCacheGetUserSession(uid uint64) (*cacheUserSession, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserSessionRecord(context.Background(), &pb.CacheGetUserSessionRecordReq{
		Uid: uid,
		Fields: []pb.CacheUserSessionField{
			pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
			pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
		},
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			if s.Code() == grpccodes.NotFound {
				return &cacheUserSession{}, nil
			}
			return nil, errors.WithMessagef(err, "CacheGetUserSessionRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetUserSessionRecord uid:%d", uid)
	}
	session := &cacheUserSession{}
	for _, record := range res.GetRecords() {
		switch record.GetField() {
		case pb.CacheUserSessionField_CacheUserSessionField_GatewayKey:
			session.gatewayKey = record.GetValue()
		case pb.CacheUserSessionField_CacheUserSessionField_OnlineKey:
			session.onlineKey = record.GetValue()
		}
	}
	return session, nil
}

func unaryCacheSetUserSession(uid uint64, gatewayKey string, onlineKey string) error {
	_, err := pb.GXCacheServiceService.CacheSetUserSessionRecord(context.Background(), &pb.CacheSetUserSessionRecordReq{
		Uid: uid,
		Records: []*pb.CacheUserSessionRecord{
			{
				Field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
				Value: gatewayKey,
			},
			{
				Field: pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
				Value: onlineKey,
			},
			{
				Field: pb.CacheUserSessionField_CacheUserSessionField_LoginTime,
				Value: strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
		},
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "CacheSetUserSessionRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetUserSessionRecord uid:%d", uid)
	}
	return nil
}

func unaryCacheDelUserSession(uid uint64) error {
	_, err := pb.GXCacheServiceService.CacheDelUserSessionRecord(context.Background(), &pb.CacheDelUserSessionRecordReq{
		Uid: uid,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "CacheDelUserSessionRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheDelUserSessionRecord uid:%d", uid)
	}
	return nil
}
