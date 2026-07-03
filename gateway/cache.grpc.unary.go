package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const (
	accountSessionExpireSecond uint64 = 5 * 60 // 用户在线 session TTL，单位秒。
)

// unaryCacheGetAccountSession 从 cache 读取用户当前在线 session。
// cache 返回 NotFound 时表示当前无在线态，gateway 按 nil session 继续登录流程。
func unaryCacheGetAccountSession(aid uint64) (*pb.CacheAccountSession, error) {
	res, err := pb.GXCacheServiceService.CacheGetAccountSession(context.Background(),
		&pb.CacheGetAccountSessionReq{
			Aid: aid,
		},
	)
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			if s.Code() == grpccodes.NotFound {
				return nil, nil
			}
			return nil, errors.WithMessagef(err, "CacheGetAccountSession aid:%d, code:%v, message:%s", aid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetAccountSession aid:%d", aid)
	}
	return res.GetSession(), nil
}

// unaryCacheBeginAccountSession 使用 CAS 创建或替换用户在线 session。
// expected 为空时要求 cache 中没有旧 session；非空时要求旧 accountSession 匹配。
func unaryCacheBeginAccountSession(aid uint64, expectedAccountSession string, session *pb.CacheAccountSession) error {
	_, err := pb.GXCacheServiceService.CacheBeginAccountSessionCAS(context.Background(),
		&pb.CacheBeginAccountSessionCASReq{
			Aid:                    aid,
			ExpectedAccountSession: expectedAccountSession,
			GatewayKey:             session.GetGatewayKey(),
			AccountSession:         session.GetAccountSession(),
			LoginTimestampMs:       session.GetLoginTimestampMs(),
			OnlineKey:              session.GetOnlineKey(),
			ExpireSecond:           accountSessionExpireSecond,
		},
	)
	return normalizeCacheSessionError(err, "CacheBeginAccountSessionCAS", aid)
}

// unaryCacheEndAccountSession 在 accountSession 匹配时删除 cache 在线 session。
func unaryCacheEndAccountSession(aid uint64, expectedAccountSession string) error {
	_, err := pb.GXCacheServiceService.CacheEndAccountSessionCAS(context.Background(), &pb.CacheEndAccountSessionCASReq{
		Aid:                    aid,
		ExpectedAccountSession: expectedAccountSession,
	})
	return normalizeCacheSessionError(err, "CacheEndAccountSessionCAS", aid)
}

// unaryCacheRefreshAccountSession 在 accountSession 匹配时刷新 cache 在线 session TTL。
func unaryCacheRefreshAccountSession(aid uint64, expectedAccountSession string) error {
	_, err := pb.GXCacheServiceService.CacheRefreshAccountSessionCAS(context.Background(), &pb.CacheRefreshAccountSessionCASReq{
		Aid:                    aid,
		ExpectedAccountSession: expectedAccountSession,
		ExpireSecond:           accountSessionExpireSecond,
	})
	return normalizeCacheSessionError(err, "CacheRefreshAccountSessionCAS", aid)
}

// normalizeCacheSessionError 统一 cache session RPC 错误语义。
// Aborted/NotFound 直接透传为业务可判断的 gRPC status，其它错误附加调用上下文。
func normalizeCacheSessionError(err error, name string, aid uint64) error {
	if err == nil {
		return nil
	}
	if s, ok := grpcstatus.FromError(err); ok {
		if s.Code() == grpccodes.Aborted || s.Code() == grpccodes.NotFound {
			return grpcstatus.Error(s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "%s aid:%d, code:%v, message:%s", name, aid, s.Code(), s.Message())
	}
	return errors.WithMessagef(err, "%s aid:%d", name, aid)
}
