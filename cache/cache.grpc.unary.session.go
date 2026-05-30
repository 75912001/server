package main

import (
	"context"
	"server/common"
	"time"

	xerror "github.com/75912001/xlib/error"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	pb "server/proto/pb"

	"github.com/redis/go-redis/v9"
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

func (s *cacheGRPCServer) CacheSetUserSessionRecord(ctx context.Context, req *pb.CacheSetUserSessionRecordReq) (*pb.CacheSetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqRecords := req.GetRecords()
	if uid == 0 || len(reqRecords) == 0 {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	records := make(map[string]string, len(reqRecords))
	for _, record := range reqRecords {
		field := cacheUserSessionFieldName(record.GetField())
		if field == "" {
			return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
		}
		records[field] = record.GetValue()
	}
	if err := GRedis.SetUserSessionRecord(ctx, uid, records); err != nil {
		return &pb.CacheSetUserSessionRecordRes{}, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	return &pb.CacheSetUserSessionRecordRes{}, nil
}

func (s *cacheGRPCServer) CacheSetUserSessionExpire(ctx context.Context, req *pb.CacheSetUserSessionExpireReq) (*pb.CacheSetUserSessionExpireRes, error) {
	uid := req.GetUid()
	expireSecond := req.GetExpireSecond()
	if uid == 0 || expireSecond == 0 {
		return &pb.CacheSetUserSessionExpireRes{
			Code: common.ECCacheInvalidArgument.Code(),
			Msg:  common.ECCacheInvalidArgument.Desc(),
			Ok:   false,
		}, nil
	}
	ok, err := GRedis.SetUserSessionExpire(ctx, uid, time.Duration(expireSecond)*time.Second)
	if err != nil {
		return &pb.CacheSetUserSessionExpireRes{
			Code: common.ECCacheRedisError.Code(),
			Msg:  errors.WithMessagef(err, "%v %v", common.ECCacheRedisError.Desc(), xruntime.Location()).Error(),
			Ok:   false,
		}, nil
	}
	return &pb.CacheSetUserSessionExpireRes{
		Code: xerror.Success.Code(),
		Msg:  xerror.Success.Desc(),
		Ok:   ok,
	}, nil
}

func (s *cacheGRPCServer) CacheGetUserSessionRecord(ctx context.Context, req *pb.CacheGetUserSessionRecordReq) (*pb.CacheGetUserSessionRecordRes, error) {
	uid := req.GetUid()
	reqFields := req.GetFields()
	if uid == 0 || len(reqFields) == 0 {
		return &pb.CacheGetUserSessionRecordRes{
			Code: common.ECCacheInvalidArgument.Code(),
			Msg:  common.ECCacheInvalidArgument.Desc(),
		}, nil
	}
	fields := make([]string, 0, len(reqFields))
	for _, reqField := range reqFields {
		field := cacheUserSessionFieldName(reqField)
		if field == "" {
			return &pb.CacheGetUserSessionRecordRes{
				Code: common.ECCacheInvalidArgument.Code(),
				Msg:  common.ECCacheInvalidArgument.Desc(),
			}, nil
		}
		fields = append(fields, field)
	}
	values, err := GRedis.GetUserSessionRecord(ctx, uid, fields)
	if err != nil {
		return &pb.CacheGetUserSessionRecordRes{
			Code: common.ECCacheRedisError.Code(),
			Msg:  errors.WithMessagef(err, "%v %v", common.ECCacheRedisError.Desc(), xruntime.Location()).Error(),
		}, nil
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
		return &pb.CacheGetUserSessionRecordRes{
			Code: common.ECCacheKeyNotFound.Code(),
			Msg:  common.ECCacheKeyNotFound.Desc(),
		}, nil
	}
	return &pb.CacheGetUserSessionRecordRes{
		Code:    xerror.Success.Code(),
		Msg:     xerror.Success.Desc(),
		Records: records,
	}, nil
}
