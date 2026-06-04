package main

import (
	"strconv"
	"testing"

	pb "server/proto/pb"
)

func BenchmarkUserSession2Map(b *testing.B) {
	records := []*pb.CacheUserSessionRecord{
		{Field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey, Value: "gateway-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_OnlineKey, Value: "online-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_UserSession, Value: "session-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_GatewaySession, Value: "gateway-session-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_LoginTime, Value: "123456"},
	}

	for b.Loop() {
		userSession2Map(records, userSessionIdentityFields...)
	}
}

func BenchmarkBuildUserSessionRecordResponse(b *testing.B) {
	reqFields := []pb.CacheUserSessionField{
		pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
		pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
		pb.CacheUserSessionField_CacheUserSessionField_UserSession,
		pb.CacheUserSessionField_CacheUserSessionField_GatewaySession,
		pb.CacheUserSessionField_CacheUserSessionField_LoginTime,
	}
	fields := []string{
		userSessionFieldGatewayKey,
		userSessionFieldOnlineKey,
		userSessionFieldUserSession,
		userSessionFieldGatewaySession,
		userSessionFieldLoginTime,
	}
	values := make(map[string]string, len(fields))
	for i, field := range fields {
		values[field] = strconv.Itoa(i)
	}

	for b.Loop() {
		buildUserSessionRecordResponse(reqFields, fields, values)
	}
}
