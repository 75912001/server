package main

import (
	"context"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"
)

const (
	// Redis hash 字段名必须和 online 侧写入/读取的 session 字段语义一致。
	userSessionFieldGatewayKey     = "gatewayKey"
	userSessionFieldOnlineKey      = "onlineKey"
	userSessionFieldUserSession    = "userSession"
	userSessionFieldGatewaySession = "gatewaySession"
	userSessionFieldLoginTime      = "loginTime"
)

var (
	// 稳定 identity 不包含 gatewaySession，避免心跳轮换影响离线/续期。
	userSessionIdentityFields = []string{
		userSessionFieldGatewayKey,
		userSessionFieldOnlineKey,
		userSessionFieldUserSession,
	}
	// 完整 session 字段用于替换在线态，缺少任意字段都视为参数不完整。
	userSessionFullFields = []string{
		userSessionFieldGatewayKey,
		userSessionFieldOnlineKey,
		userSessionFieldUserSession,
		userSessionFieldGatewaySession,
		userSessionFieldLoginTime,
	}
)

// genUserSessionFieldName 将 proto enum 转成 Redis hash 字段名。
// 返回空字符串表示请求携带了未支持或未指定的字段。
func genUserSessionFieldName(field pb.CacheUserSessionField) string {
	switch field {
	case pb.CacheUserSessionField_CacheUserSessionField_GatewayKey:
		return userSessionFieldGatewayKey
	case pb.CacheUserSessionField_CacheUserSessionField_OnlineKey:
		return userSessionFieldOnlineKey
	case pb.CacheUserSessionField_CacheUserSessionField_UserSession:
		return userSessionFieldUserSession
	case pb.CacheUserSessionField_CacheUserSessionField_GatewaySession:
		return userSessionFieldGatewaySession
	case pb.CacheUserSessionField_CacheUserSessionField_LoginTime:
		return userSessionFieldLoginTime
	default:
		return ""
	}
}

// userSession2Map 将请求 records 转成 Redis HSET/HGET 使用的字段 map。
// requiredFields 只要求字段存在，字段值允许为空字符串，用于首次登录空 identity 的 CAS。
func userSession2Map(reqRecords []*pb.CacheUserSessionRecord, requiredFields ...string) (map[string]string, bool) {
	records := make(map[string]string, len(reqRecords))
	for _, record := range reqRecords {
		field := genUserSessionFieldName(record.GetField())
		if field == "" {
			return nil, false
		}
		records[field] = record.GetValue()
	}
	return records, hasUserSessionField(records, requiredFields...)
}

// hasUserSessionField 只校验字段名是否存在，不校验 value 是否为空。
func hasUserSessionField(records map[string]string, fields ...string) bool {
	for _, field := range fields {
		if _, ok := records[field]; !ok {
			return false
		}
	}
	return true
}

// userSessionField2Slice 将请求 fields 转成 Redis HMGET 字段列表。
// 返回字段列表保持请求顺序，便于响应时按调用方要求排序。
func userSessionField2Slice(reqFields []pb.CacheUserSessionField) ([]string, bool) {
	fields := make([]string, 0, len(reqFields))
	for _, reqField := range reqFields {
		field := genUserSessionFieldName(reqField)
		if field == "" {
			return nil, false
		}
		fields = append(fields, field)
	}
	return fields, true
}

// buildUserSessionRecordResponse 根据请求字段顺序组装响应。
// Redis 中不存在的字段会被跳过；如果全部缺失，handler 返回 NotFound。
func buildUserSessionRecordResponse(reqFields []pb.CacheUserSessionField, fields []string, values map[string]string) []*pb.CacheUserSessionRecord {
	records := make([]*pb.CacheUserSessionRecord, 0, len(reqFields))
	for i, field := range fields {
		value, ok := values[field]
		if !ok {
			continue
		}
		records = append(records, &pb.CacheUserSessionRecord{
			Field: reqFields[i],
			Value: value,
		})
	}
	return records
}

// CacheSetUserSessionRecord 直接写入用户在线 session hash 字段。
// 该接口不做 CAS 校验，通常只用于明确允许覆盖指定字段的场景。
func (s *cacheGRPCServer) CacheSetUserSessionRecord(ctx context.Context, req *pb.CacheSetUserSessionRecordReq) (*pb.CacheSetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqRecords := req.GetRecords()
	if uid == 0 || len(reqRecords) == 0 {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records, ok := userSession2Map(reqRecords)
	if !ok {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := GRedis.SetUserSessionRecord(ctx, uid, records); err != nil {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetUserSessionRecordRes{}, nil
}

// CacheDelUserSessionRecord 删除用户在线 session。
// expectedRecords 必须包含稳定 identity，Redis 层只有在 expected 完全匹配时才会删除。
func (s *cacheGRPCServer) CacheDelUserSessionRecord(ctx context.Context, req *pb.CacheDelUserSessionRecordReq) (*pb.CacheDelUserSessionRecordRes, error) {
	uid := req.GetUid()
	expectedRecords := req.GetExpectedRecords()
	if uid == 0 || len(expectedRecords) == 0 {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expected, ok := userSession2Map(expectedRecords, userSessionIdentityFields...)
	if !ok {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	deleted, err := GRedis.DelUserSessionRecord(ctx, uid, expected)
	if err != nil {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !deleted { // CAS 失败说明在线态已经被其他登录、离线或心跳流程接管。
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Aborted, "user gatewaySession changed")
	}
	return &pb.CacheDelUserSessionRecordRes{}, nil
}

// CacheReplaceUserSessionRecord 原子替换用户在线 session。
// expectedRecords 匹配旧 identity 后，records 必须携带完整 session 字段并一次性写入。
func (s *cacheGRPCServer) CacheReplaceUserSessionRecord(ctx context.Context, req *pb.CacheReplaceUserSessionRecordReq) (*pb.CacheReplaceUserSessionRecordRes, error) {
	uid := req.GetUid()
	if uid == 0 || len(req.GetExpectedRecords()) == 0 || len(req.GetRecords()) == 0 {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expected, ok := userSession2Map(req.GetExpectedRecords(), userSessionIdentityFields...)
	if !ok {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records, ok := userSession2Map(req.GetRecords(), userSessionFullFields...)
	if !ok {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	replaced, err := GRedis.ReplaceUserSessionRecord(ctx, uid, expected, records, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !replaced { // CAS 失败时不写入新 session，调用方需要重新读取在线态后再决策。
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Aborted, "user gatewaySession changed")
	}
	return &pb.CacheReplaceUserSessionRecordRes{}, nil
}

// CacheSetUserSessionExpire 刷新用户在线 session TTL。
// 仅在 expectedRecords 匹配稳定 identity 时刷新。
func (s *cacheGRPCServer) CacheSetUserSessionExpire(ctx context.Context, req *pb.CacheSetUserSessionExpireReq) (*pb.CacheSetUserSessionExpireRes, error) {
	uid := req.GetUid()
	expireSecond := req.GetExpireSecond()
	expectedRecords := req.GetExpectedRecords()
	if uid == 0 || expireSecond == 0 || len(expectedRecords) == 0 {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expected, valid := userSession2Map(expectedRecords, userSessionIdentityFields...)
	if !valid {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	ok, err := GRedis.SetUserSessionExpire(ctx, uid, time.Duration(expireSecond)*time.Second, expected)
	if err != nil {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.Aborted, "user gatewaySession changed")
	}
	return &pb.CacheSetUserSessionExpireRes{}, nil
}

// CacheGetUserSessionRecord 按请求字段读取用户在线 session。
// 响应只包含 Redis 中存在的字段；请求字段全部缺失时返回 NotFound。
func (s *cacheGRPCServer) CacheGetUserSessionRecord(ctx context.Context, req *pb.CacheGetUserSessionRecordReq) (*pb.CacheGetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqFields := req.GetFields()
	if uid == 0 || len(reqFields) == 0 {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	fields, ok := userSessionField2Slice(reqFields)
	if !ok {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	values, err := GRedis.GetUserSessionRecord(ctx, uid, fields)
	if err != nil {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	records := buildUserSessionRecordResponse(reqFields, fields, values)
	if len(records) == 0 {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.NotFound, "user gatewaySession record not exist")
	}
	return &pb.CacheGetUserSessionRecordRes{
		Records: records,
	}, nil
}
