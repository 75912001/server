package main

import (
	"context"
	"strconv"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"
)

const (
	// Redis 哈希字段名对应 user:{uid}:session。
	userSessionFieldGatewayKey  = "gatewayKey"
	userSessionFieldUserSession = "userSession" // 身份字段用于判断 CAS 操作的预期值是否匹配
	userSessionFieldLoginTime   = "loginTime"
	userSessionFieldOnlineKey   = "onlineKey"
)

// cacheUserSessionRecordMap 将完整在线会话转成 Redis 哈希字段。
func cacheUserSessionRecordMap(gatewayKey string, userSession string, loginTime int64, onlineKey string) (map[string]string, bool) {
	if gatewayKey == "" || userSession == "" || loginTime == 0 || onlineKey == "" {
		return nil, false
	}
	records := map[string]string{
		userSessionFieldUserSession: userSession,
	}
	records[userSessionFieldGatewayKey] = gatewayKey
	records[userSessionFieldLoginTime] = formatUserSessionLoginTime(loginTime)
	records[userSessionFieldOnlineKey] = onlineKey
	return records, true
}

// cacheUserSessionFromMap 从 Redis 哈希字段还原完整在线会话。
func cacheUserSessionFromMap(records map[string]string) (*pb.CacheUserSession, bool) {
	if len(records) == 0 {
		return nil, false
	}
	loginTime, ok := parseUserSessionLoginTime(records[userSessionFieldLoginTime])
	if !ok {
		return nil, false
	}
	session := &pb.CacheUserSession{
		GatewayKey:  records[userSessionFieldGatewayKey],
		UserSession: records[userSessionFieldUserSession],
		LoginTime:   loginTime,
		OnlineKey:   records[userSessionFieldOnlineKey],
	}
	if session.GetGatewayKey() == "" || session.GetUserSession() == "" || session.GetLoginTime() == 0 || session.GetOnlineKey() == "" {
		return nil, false
	}
	return session, true
}

func formatUserSessionLoginTime(loginTime int64) string {
	return strconv.FormatInt(loginTime, 10)
}

func parseUserSessionLoginTime(value string) (int64, bool) {
	loginTime, err := strconv.ParseInt(value, 10, 64)
	return loginTime, err == nil && loginTime != 0
}

// CacheGetUserSession 读取指定 uid 当前完整在线会话。
func (s *cacheGRPCServer) CacheGetUserSession(ctx context.Context, req *pb.CacheGetUserSessionReq) (*pb.CacheGetUserSessionRes, error) {
	uid := req.GetUid()
	if uid == 0 {
		return &pb.CacheGetUserSessionRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	values, err := GRedis.GetUserSession(ctx, uid)
	if err != nil {
		return &pb.CacheGetUserSessionRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	session, ok := cacheUserSessionFromMap(values)
	if !ok {
		return &pb.CacheGetUserSessionRes{}, grpcstatus.Error(grpccodes.NotFound, "user session not found")
	}
	return &pb.CacheGetUserSessionRes{Session: session}, nil
}

// CacheBeginUserSessionCAS 仅在预期身份仍匹配时创建或替换在线会话。
func (s *cacheGRPCServer) CacheBeginUserSessionCAS(ctx context.Context, req *pb.CacheBeginUserSessionCASReq) (*pb.CacheBeginUserSessionCASRes, error) {
	uid := req.GetUid()
	if uid == 0 || req.GetExpireSecond() == 0 {
		return &pb.CacheBeginUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	session, ok := cacheUserSessionRecordMap(req.GetGatewayKey(), req.GetUserSession(), req.GetLoginTime(), req.GetOnlineKey())
	if !ok {
		return &pb.CacheBeginUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	created, err := GRedis.BeginUserSessionCAS(ctx, uid, req.GetExpectedUserSession(), session, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheBeginUserSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !created {
		return &pb.CacheBeginUserSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "user session changed")
	}
	return &pb.CacheBeginUserSessionCASRes{}, nil
}

// CacheEndUserSessionCAS 仅在预期身份仍匹配时删除在线会话。
func (s *cacheGRPCServer) CacheEndUserSessionCAS(ctx context.Context, req *pb.CacheEndUserSessionCASReq) (*pb.CacheEndUserSessionCASRes, error) {
	uid := req.GetUid()
	if uid == 0 {
		return &pb.CacheEndUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedUserSession := req.GetExpectedUserSession()
	if expectedUserSession == "" {
		return &pb.CacheEndUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	deleted, err := GRedis.EndUserSessionCAS(ctx, uid, expectedUserSession)
	if err != nil {
		return &pb.CacheEndUserSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !deleted {
		return &pb.CacheEndUserSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "user session changed")
	}
	return &pb.CacheEndUserSessionCASRes{}, nil
}

// CacheRefreshUserSessionCAS 仅在预期身份仍匹配时刷新在线会话 TTL。
func (s *cacheGRPCServer) CacheRefreshUserSessionCAS(ctx context.Context, req *pb.CacheRefreshUserSessionCASReq) (*pb.CacheRefreshUserSessionCASRes, error) {
	uid := req.GetUid()
	if uid == 0 || req.GetExpireSecond() == 0 {
		return &pb.CacheRefreshUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedUserSession := req.GetExpectedUserSession()
	if expectedUserSession == "" {
		return &pb.CacheRefreshUserSessionCASRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	refreshed, err := GRedis.RefreshUserSessionCAS(ctx, uid, expectedUserSession, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheRefreshUserSessionCASRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !refreshed {
		return &pb.CacheRefreshUserSessionCASRes{}, grpcstatus.Error(grpccodes.Aborted, "user session changed")
	}
	return &pb.CacheRefreshUserSessionCASRes{}, nil
}
