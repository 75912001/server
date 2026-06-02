package main

import (
	"context"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"
)

func cacheUserSessionFieldName(field pb.CacheUserSessionField) string {
	switch field {
	case pb.CacheUserSessionField_CacheUserSessionField_GatewayKey:
		return "gatewayKey"
	case pb.CacheUserSessionField_CacheUserSessionField_OnlineKey:
		return "onlineKey"
	case pb.CacheUserSessionField_CacheUserSessionField_Session:
		return "session"
	case pb.CacheUserSessionField_CacheUserSessionField_LoginTime:
		return "loginTime"
	default:
		return ""
	}
}

func cacheUserSessionRecords(reqRecords []*pb.CacheUserSessionRecord) (map[string]string, bool) {
	records := make(map[string]string, len(reqRecords))
	for _, record := range reqRecords {
		field := cacheUserSessionFieldName(record.GetField())
		if field == "" {
			return nil, false
		}
		records[field] = record.GetValue()
	}
	return records, true
}

func cacheUserSessionRecordsHas(records map[string]string, fields ...string) bool {
	for _, field := range fields {
		if _, ok := records[field]; !ok {
			return false
		}
	}
	return true
}

func (s *cacheGRPCServer) CacheSetUserSessionRecord(ctx context.Context, req *pb.CacheSetUserSessionRecordReq) (*pb.CacheSetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqRecords := req.GetRecords()
	if uid == 0 || len(reqRecords) == 0 {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records, ok := cacheUserSessionRecords(reqRecords)
	if !ok {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := GRedis.SetUserSessionRecord(ctx, uid, records); err != nil {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetUserSessionRecordRes{}, nil
}

func (s *cacheGRPCServer) CacheDelUserSessionRecord(ctx context.Context, req *pb.CacheDelUserSessionRecordReq) (*pb.CacheDelUserSessionRecordRes, error) {
	uid := req.GetUid()
	if uid == 0 {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedRecords := req.GetExpectedRecords()
	var expected map[string]string
	if len(expectedRecords) == 0 {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	var ok bool
	expected, ok = cacheUserSessionRecords(expectedRecords)
	if !ok || !cacheUserSessionRecordsHas(expected, "gatewayKey", "onlineKey", "session") {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := GRedis.DelUserSessionRecord(ctx, uid, expected); err != nil {
		return &pb.CacheDelUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheDelUserSessionRecordRes{}, nil
}

func (s *cacheGRPCServer) CacheReplaceUserSessionRecord(ctx context.Context, req *pb.CacheReplaceUserSessionRecordReq) (*pb.CacheReplaceUserSessionRecordRes, error) {
	uid := req.GetUid()
	if uid == 0 || len(req.GetExpectedRecords()) == 0 || len(req.GetRecords()) == 0 {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expected, ok := cacheUserSessionRecords(req.GetExpectedRecords())
	if !ok || !cacheUserSessionRecordsHas(expected, "gatewayKey", "onlineKey", "session") {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records, ok := cacheUserSessionRecords(req.GetRecords())
	if !ok || !cacheUserSessionRecordsHas(records, "gatewayKey", "onlineKey", "session", "loginTime") {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	replaced, err := GRedis.ReplaceUserSessionRecord(ctx, uid, expected, records, req.GetExpireSecond())
	if err != nil {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !replaced {
		return &pb.CacheReplaceUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Aborted, "user session changed")
	}
	return &pb.CacheReplaceUserSessionRecordRes{}, nil
}

func (s *cacheGRPCServer) CacheSetUserSessionExpire(ctx context.Context, req *pb.CacheSetUserSessionExpireReq) (*pb.CacheSetUserSessionExpireRes, error) {
	uid := req.GetUid()
	expireSecond := req.GetExpireSecond()
	if uid == 0 || expireSecond == 0 {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	expectedRecords := req.GetExpectedRecords()
	var expected map[string]string
	if len(expectedRecords) != 0 {
		var valid bool
		expected, valid = cacheUserSessionRecords(expectedRecords)
		if !valid || !cacheUserSessionRecordsHas(expected, "gatewayKey", "onlineKey", "session") {
			return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
		}
	}
	ok, err := GRedis.SetUserSessionExpire(ctx, uid, time.Duration(expireSecond)*time.Second, expected)
	if err != nil {
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if !ok {
		if len(expected) != 0 {
			return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.Aborted, "user session changed")
		}
		return &pb.CacheSetUserSessionExpireRes{}, grpcstatus.Error(grpccodes.NotFound, "user session not exist")
	}
	return &pb.CacheSetUserSessionExpireRes{}, nil
}

func (s *cacheGRPCServer) CacheGetUserSessionRecord(ctx context.Context, req *pb.CacheGetUserSessionRecordReq) (*pb.CacheGetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqFields := req.GetFields()
	if uid == 0 || len(reqFields) == 0 {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	fields := make([]string, 0, len(reqFields))
	for _, reqField := range reqFields {
		field := cacheUserSessionFieldName(reqField)
		if field == "" {
			return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
		}
		fields = append(fields, field)
	}
	values, err := GRedis.GetUserSessionRecord(ctx, uid, fields)
	if err != nil {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
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
	if len(records) == 0 {
		return &pb.CacheGetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.NotFound, "user session record not exist")
	}
	return &pb.CacheGetUserSessionRecordRes{
		Records: records,
	}, nil
}
