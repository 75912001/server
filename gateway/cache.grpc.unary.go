package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const (
	userSessionExpireSecond uint64 = 5 * 60 // 用户在线 session TTL，单位秒。
)

// unaryCacheGetUserSession 从 cache 读取用户当前在线 session。
// cache 返回 NotFound 时表示当前无在线态，gateway 按 nil session 继续登录流程。
func unaryCacheGetUserSession(uid uint64) (*pb.CacheUserSession, error) {
	res, err := pb.GXCacheServiceService.CacheGetUserSession(context.Background(),
		&pb.CacheGetUserSessionReq{
			Uid: uid,
		},
	)
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			if s.Code() == grpccodes.NotFound {
				return nil, nil
			}
			return nil, errors.WithMessagef(err, "CacheGetUserSession uid:%d, code:%v, message:%s", uid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetUserSession uid:%d", uid)
	}
	return res.GetSession(), nil
}

// unaryCacheBeginUserSession 使用 CAS 创建或替换用户在线 session。
// expected 为空时要求 cache 中没有旧 session；非空时要求旧 userSession 匹配。
func unaryCacheBeginUserSession(uid uint64, expectedUserSession string, session *pb.CacheUserSession) error {
	_, err := pb.GXCacheServiceService.CacheBeginUserSessionCAS(context.Background(),
		&pb.CacheBeginUserSessionCASReq{
			Uid:                 uid,
			ExpectedUserSession: expectedUserSession,
			GatewayKey:          session.GetGatewayKey(),
			UserSession:         session.GetUserSession(),
			LoginTimeMs:         session.GetLoginTimeMs(),
			OnlineKey:           session.GetOnlineKey(),
			ExpireSecond:        userSessionExpireSecond,
		},
	)
	return normalizeCacheSessionError(err, "CacheBeginUserSessionCAS", uid)
}

// unaryCacheEndUserSession 在 userSession 匹配时删除 cache 在线 session。
func unaryCacheEndUserSession(uid uint64, expectedUserSession string) error {
	_, err := pb.GXCacheServiceService.CacheEndUserSessionCAS(context.Background(), &pb.CacheEndUserSessionCASReq{
		Uid:                 uid,
		ExpectedUserSession: expectedUserSession,
	})
	return normalizeCacheSessionError(err, "CacheEndUserSessionCAS", uid)
}

// unaryCacheRefreshUserSession 在 userSession 匹配时刷新 cache 在线 session TTL。
func unaryCacheRefreshUserSession(uid uint64, expectedUserSession string) error {
	_, err := pb.GXCacheServiceService.CacheRefreshUserSessionCAS(context.Background(), &pb.CacheRefreshUserSessionCASReq{
		Uid:                 uid,
		ExpectedUserSession: expectedUserSession,
		ExpireSecond:        userSessionExpireSecond,
	})
	return normalizeCacheSessionError(err, "CacheRefreshUserSessionCAS", uid)
}

// normalizeCacheSessionError 统一 cache session RPC 错误语义。
// Aborted/NotFound 直接透传为业务可判断的 gRPC status，其它错误附加调用上下文。
func normalizeCacheSessionError(err error, name string, uid uint64) error {
	if err == nil {
		return nil
	}
	if s, ok := grpcstatus.FromError(err); ok {
		if s.Code() == grpccodes.Aborted || s.Code() == grpccodes.NotFound {
			return grpcstatus.Error(s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "%s uid:%d, code:%v, message:%s", name, uid, s.Code(), s.Message())
	}
	return errors.WithMessagef(err, "%s uid:%d", name, uid)
}
