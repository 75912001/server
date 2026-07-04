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

func robotAccount(aid uint64) string {
	return fmt.Sprintf("robot.%d", aid)
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

func cacheEnsureRobotAccountRecord(aid uint64) error {
	account := robotAccount(aid)
	res, err := pb.GXCacheServiceService.CacheGetAccountRecord(context.Background(), &pb.CacheGetAccountRecordReq{
		Aid: aid,
	})
	if err == nil {
		accountRecord := res.GetAccountRecord()
		if accountRecord == nil {
			return errors.Errorf("CacheGetAccountRecord aid:%d account record is nil", aid)
		}
		if accountRecord.GetAccount() != account {
			return errors.Errorf("CacheGetAccountRecord aid:%d account mismatch current:%s expect:%s", aid, accountRecord.GetAccount(), account)
		}
		return nil
	}
	s, ok := grpcstatus.FromError(err)
	if !ok || s.Code() != codes.NotFound {
		if ok {
			return errors.WithMessagef(err, "CacheGetAccountRecord aid:%d code:%v message:%s", aid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheGetAccountRecord aid:%d", aid)
	}
	_, err = pb.GXCacheServiceService.CacheSetAccountRecord(context.Background(), &pb.CacheSetAccountRecordReq{
		Aid: aid,
		AccountRecord: &pb.AccountRecord{
			Aid:                            aid,
			Account:                        account,
			AccountCreateTimestampMs:       time.Now().UnixMilli(),
			AccountRecordCreateTimestampMs: 0,
		},
	})
	if err != nil {
		s, ok = grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d account:%s code:%v message:%s", aid, account, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d account:%s", aid, account)
	}
	return nil
}
