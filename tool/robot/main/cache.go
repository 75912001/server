package main

import (
	"context"
	"fmt"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func robotAccount(uid uint64) string {
	return fmt.Sprintf("robot.%d", uid)
}

func newAccountVerifyToken(account string) string {
	return fmt.Sprintf("%s.%d", account, time.Now().UnixNano())
}

func cacheSetAccountVerifyToken(account string) (string, error) {
	if account == "" {
		return "", xerror.InvalidArgument
	}
	accountVerifyToken := newAccountVerifyToken(account)
	_, err := pb.GXCacheServiceService.CacheSetAccountVerifyToken(context.Background(), &pb.CacheSetAccountVerifyTokenReq{
		Account:            account,
		AccountVerifyToken: accountVerifyToken,
		ExpireSecond:       GConfigYaml.CacheAccountVerifyTokenExpire,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return "", errors.WithMessagef(err, "CacheSetAccountVerifyToken account:%s accountVerifyToken:%s code:%v message:%s", account, accountVerifyToken, s.Code(), s.Message())
		}
		return "", errors.WithMessagef(err, "CacheSetAccountVerifyToken account:%s accountVerifyToken:%s", account, accountVerifyToken)
	}
	return accountVerifyToken, nil
}

func waitCache(timeout time.Duration) error {
	if GDiscoveredCacheMgr.m.Len() > 0 {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if GDiscoveredCacheMgr.m.Len() > 0 {
				return nil
			}
		case <-timer.C:
			return errors.WithMessage(xerror.Timeout, "wait cache timeout")
		}
	}
}

func cacheEnsureRobotUserRecord(uid uint64) error {
	account := robotAccount(uid)
	res, err := pb.GXCacheServiceService.CacheGetUserRecord(context.Background(), &pb.CacheGetUserRecordReq{
		Uid: uid,
	})
	if err == nil {
		userRecord := res.GetUserRecord()
		if userRecord == nil {
			return errors.Errorf("CacheGetUserRecord uid:%d user record is nil", uid)
		}
		if userRecord.GetAccount() != account {
			return errors.Errorf("CacheGetUserRecord uid:%d account mismatch current:%s expect:%s", uid, userRecord.GetAccount(), account)
		}
		return nil
	}
	s, ok := grpcstatus.FromError(err)
	if !ok || s.Code() != codes.NotFound {
		if ok {
			return errors.WithMessagef(err, "CacheGetUserRecord uid:%d code:%v message:%s", uid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheGetUserRecord uid:%d", uid)
	}
	_, err = pb.GXCacheServiceService.CacheSetUserRecord(context.Background(), &pb.CacheSetUserRecordReq{
		Uid: uid,
		UserRecord: &pb.UserRecord{
			Uid:                      uid,
			Account:                  account,
			AccountCreateTimestampMs: time.Now().UnixMilli(),
			UserCreateTimestampMs:    0,
		},
	})
	if err != nil {
		s, ok = grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "CacheSetUserRecord uid:%d account:%s code:%v message:%s", uid, account, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetUserRecord uid:%d account:%s", uid, account)
	}
	return nil
}
