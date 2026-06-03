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

const (
	userSessionExpireSecond  uint64 = 5 * 60
	userSessionRefreshSecond uint64 = 4 * 60
)

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

func unaryCacheSetUserRecord(uid uint64, userRecord *pb.UserRecord) error {
	_, err := pb.GXCacheServiceService.CacheSetUserRecord(context.Background(), &pb.CacheSetUserRecordReq{
		Uid:        uid,
		UserRecord: userRecord,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "CacheSetUserRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetUserRecord uid:%d", uid)
	}
	return nil
}

type cacheUserSession struct {
	gatewayKey     string
	onlineKey      string
	gatewaySession string
	loginTime      string
}

func unaryCacheGetUserSession(uid uint64) (*cacheUserSession, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserSessionRecord(context.Background(), &pb.CacheGetUserSessionRecordReq{
		Uid: uid,
		Fields: []pb.CacheUserSessionField{
			pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
			pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
			pb.CacheUserSessionField_CacheUserSessionField_GatewaySession,
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
		case pb.CacheUserSessionField_CacheUserSessionField_GatewaySession:
			session.gatewaySession = record.GetValue()
		}
	}
	return session, nil
}

func newCacheUserSession(gatewayKey string, onlineKey string, gatewaySession string) (*cacheUserSession, error) {
	return &cacheUserSession{
		gatewayKey:     gatewayKey,
		onlineKey:      onlineKey,
		gatewaySession: gatewaySession,
		loginTime:      strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, nil
}

func cacheUserSessionExpectedRecords(session *cacheUserSession) []*pb.CacheUserSessionRecord {
	return []*pb.CacheUserSessionRecord{
		{
			Field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
			Value: session.gatewayKey,
		},
		{
			Field: pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
			Value: session.onlineKey,
		},
		{
			Field: pb.CacheUserSessionField_CacheUserSessionField_GatewaySession,
			Value: session.gatewaySession,
		},
	}
}

func cacheUserSessionRecords(session *cacheUserSession) []*pb.CacheUserSessionRecord {
	records := cacheUserSessionExpectedRecords(session)
	records = append(records, &pb.CacheUserSessionRecord{
		Field: pb.CacheUserSessionField_CacheUserSessionField_LoginTime,
		Value: session.loginTime,
	})
	return records
}

func unaryCacheReplaceUserSession(uid uint64, expected *cacheUserSession, session *cacheUserSession) error {
	_, err := pb.GXCacheServiceService.CacheReplaceUserSessionRecord(context.Background(), &pb.CacheReplaceUserSessionRecordReq{
		Uid:             uid,
		ExpectedRecords: cacheUserSessionExpectedRecords(expected),
		Records:         cacheUserSessionRecords(session),
		ExpireSecond:    userSessionExpireSecond,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			if s.Code() == grpccodes.Aborted {
				return grpcstatus.Error(grpccodes.Aborted, s.Message())
			}
			return errors.WithMessagef(err, "CacheReplaceUserSessionRecord uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheReplaceUserSessionRecord uid:%d", uid)
	}
	return nil
}

func unaryCacheSetUserSessionExpire(uid uint64, expected *cacheUserSession) error {
	_, err := pb.GXCacheServiceService.CacheSetUserSessionExpire(context.Background(), &pb.CacheSetUserSessionExpireReq{
		Uid:             uid,
		ExpireSecond:    userSessionExpireSecond,
		ExpectedRecords: cacheUserSessionExpectedRecords(expected),
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			if s.Code() == grpccodes.Aborted || s.Code() == grpccodes.NotFound {
				return grpcstatus.Error(s.Code(), s.Message())
			}
			return errors.WithMessagef(err, "CacheSetUserSessionExpire uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetUserSessionExpire uid:%d", uid)
	}
	return nil
}

func unaryCacheDelUserSession(uid uint64, expected *cacheUserSession) error {
	var expectedRecords []*pb.CacheUserSessionRecord
	if expected != nil {
		expectedRecords = cacheUserSessionExpectedRecords(expected)
	}
	_, err := pb.GXCacheServiceService.CacheDelUserSessionRecord(context.Background(), &pb.CacheDelUserSessionRecordReq{
		Uid:             uid,
		ExpectedRecords: expectedRecords,
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
