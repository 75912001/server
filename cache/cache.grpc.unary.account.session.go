package main

import (
	"context"

	xtime "github.com/75912001/xlib/time"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"
)

const (
	// Redis 哈希字段名对应 account:{aid}:session。
	accountSessionFieldGatewayKey       = "gatewayKey"
	accountSessionFieldAccountSession   = "accountSession" // 身份字段用于判断 CAS 操作的预期值是否匹配
	accountSessionFieldLoginTimestampMs = "loginTimestampMs"
	accountSessionFieldOnlineKey        = "onlineKey"
)

// CacheGetAccountSession 读取指定 aid 当前完整在线会话。
func (s *cacheGRPCServer) CacheGetAccountSession(ctx context.Context, req *pb.CacheGetAccountSessionReq) (*pb.CacheGetAccountSessionRes, error) {
	aid := req.GetAid()
	if aid == 0 {
		return &pb.CacheGetAccountSessionRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	values, err := GRedis.GetAccountSession(ctx, aid)
	if err != nil {
		return &pb.CacheGetAccountSessionRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	loginTimestampMs, ok := xtime.ParseTimestampMs(values[accountSessionFieldLoginTimestampMs])
	if !ok || loginTimestampMs == 0 {
		return &pb.CacheGetAccountSessionRes{}, grpcstatus.Error(grpccodes.NotFound, "account session not found")
	}
	session := &pb.CacheAccountSession{
		GatewayKey:       values[accountSessionFieldGatewayKey],
		AccountSession:   values[accountSessionFieldAccountSession],
		LoginTimestampMs: loginTimestampMs,
		OnlineKey:        values[accountSessionFieldOnlineKey],
	}
	if session.GetGatewayKey() == "" || session.GetAccountSession() == "" || session.GetLoginTimestampMs() == 0 || session.GetOnlineKey() == "" {
		return &pb.CacheGetAccountSessionRes{}, grpcstatus.Error(grpccodes.NotFound, "account session not found")
	}
	return &pb.CacheGetAccountSessionRes{Session: session}, nil
}

// CacheBeginAccountSessionCAS 仅在预期身份仍匹配时创建或替换在线会话。
func (s *cacheGRPCServer) CacheBeginAccountSessionCAS(ctx context.Context, req *pb.CacheBeginAccountSessionCASReq) (*pb.CacheBeginAccountSessionCASRes, error) {
	aid := req.GetAid()
	if aid == 0 || req.GetExpireSecond() == 0 {
		return &pb.CacheBeginAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if req.GetGatewayKey() == "" || req.GetAccountSession() == "" || req.GetLoginTimestampMs() == 0 || req.GetOnlineKey() == "" {
		return &pb.CacheBeginAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records := map[string]string{
		accountSessionFieldGatewayKey:       req.GetGatewayKey(),
		accountSessionFieldAccountSession:   req.GetAccountSession(),
		accountSessionFieldLoginTimestampMs: xtime.FormatTimestampMs(req.GetLoginTimestampMs()),
		accountSessionFieldOnlineKey:        req.GetOnlineKey(),
	}
	created, err := GRedis.BeginAccountSessionCAS(ctx, aid, req.GetExpectedAccountSession(), records, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheBeginAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !created {
		return &pb.CacheBeginAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "account session changed")
	}
	return &pb.CacheBeginAccountSessionCASRes{}, nil
}

// CacheEndAccountSessionCAS 仅在预期身份仍匹配时删除在线会话。
func (s *cacheGRPCServer) CacheEndAccountSessionCAS(ctx context.Context, req *pb.CacheEndAccountSessionCASReq) (*pb.CacheEndAccountSessionCASRes, error) {
	aid := req.GetAid()
	if aid == 0 {
		return &pb.CacheEndAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedAccountSession := req.GetExpectedAccountSession()
	if expectedAccountSession == "" {
		return &pb.CacheEndAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	deleted, err := GRedis.EndAccountSessionCAS(ctx, aid, expectedAccountSession)
	if err != nil {
		return &pb.CacheEndAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !deleted {
		return &pb.CacheEndAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "account session changed")
	}
	return &pb.CacheEndAccountSessionCASRes{}, nil
}

// CacheRefreshAccountSessionCAS 仅在预期身份仍匹配时刷新在线会话 TTL。
func (s *cacheGRPCServer) CacheRefreshAccountSessionCAS(ctx context.Context, req *pb.CacheRefreshAccountSessionCASReq) (*pb.CacheRefreshAccountSessionCASRes, error) {
	aid := req.GetAid()
	if aid == 0 || req.GetExpireSecond() == 0 {
		return &pb.CacheRefreshAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedAccountSession := req.GetExpectedAccountSession()
	if expectedAccountSession == "" {
		return &pb.CacheRefreshAccountSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	refreshed, err := GRedis.RefreshAccountSessionCAS(ctx, aid, expectedAccountSession, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheRefreshAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !refreshed {
		return &pb.CacheRefreshAccountSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "account session changed")
	}
	return &pb.CacheRefreshAccountSessionCASRes{}, nil
}
