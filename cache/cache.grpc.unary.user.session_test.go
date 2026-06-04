package main

import (
	"reflect"
	"testing"

	pb "server/proto/pb"
)

func TestGenUserSessionFieldName(t *testing.T) {
	tests := []struct {
		field pb.CacheUserSessionField
		want  string
	}{
		{field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey, want: userSessionFieldGatewayKey},
		{field: pb.CacheUserSessionField_CacheUserSessionField_OnlineKey, want: userSessionFieldOnlineKey},
		{field: pb.CacheUserSessionField_CacheUserSessionField_UserSession, want: userSessionFieldUserSession},
		{field: pb.CacheUserSessionField_CacheUserSessionField_GatewaySession, want: userSessionFieldGatewaySession},
		{field: pb.CacheUserSessionField_CacheUserSessionField_LoginTime, want: userSessionFieldLoginTime},
		{field: pb.CacheUserSessionField_CacheUserSessionField_Unspecified, want: ""},
	}

	for _, tt := range tests {
		if got := genUserSessionFieldName(tt.field); got != tt.want {
			t.Fatalf("genUserSessionFieldName(%v) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestUserSession2Map(t *testing.T) {
	records := []*pb.CacheUserSessionRecord{
		{Field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey, Value: "gateway-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_OnlineKey, Value: "online-1"},
		{Field: pb.CacheUserSessionField_CacheUserSessionField_UserSession, Value: "session-1"},
	}

	got, ok := userSession2Map(records, userSessionIdentityFields...)
	if !ok {
		t.Fatal("userSession2Map returned false")
	}
	want := map[string]string{
		userSessionFieldGatewayKey:  "gateway-1",
		userSessionFieldOnlineKey:   "online-1",
		userSessionFieldUserSession: "session-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestUserSession2MapInvalidOrMissingRequired(t *testing.T) {
	if _, ok := userSession2Map([]*pb.CacheUserSessionRecord{
		{Field: pb.CacheUserSessionField_CacheUserSessionField_Unspecified, Value: "bad"},
	}); ok {
		t.Fatal("userSession2Map accepted invalid field")
	}

	if _, ok := userSession2Map([]*pb.CacheUserSessionRecord{
		{Field: pb.CacheUserSessionField_CacheUserSessionField_GatewayKey, Value: "gateway-1"},
	}, userSessionIdentityFields...); ok {
		t.Fatal("userSession2Map accepted missing required fields")
	}
}

func TestUserSessionField2SliceKeepsRequestOrder(t *testing.T) {
	fields, ok := userSessionField2Slice([]pb.CacheUserSessionField{
		pb.CacheUserSessionField_CacheUserSessionField_LoginTime,
		pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
		pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
	})
	if !ok {
		t.Fatal("userSessionField2Slice returned false")
	}
	want := []string{userSessionFieldLoginTime, userSessionFieldGatewayKey, userSessionFieldOnlineKey}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %#v, want %#v", fields, want)
	}
}

func TestBuildUserSessionRecordResponseSkipsMissingFields(t *testing.T) {
	reqFields := []pb.CacheUserSessionField{
		pb.CacheUserSessionField_CacheUserSessionField_GatewayKey,
		pb.CacheUserSessionField_CacheUserSessionField_OnlineKey,
		pb.CacheUserSessionField_CacheUserSessionField_LoginTime,
	}
	fields := []string{userSessionFieldGatewayKey, userSessionFieldOnlineKey, userSessionFieldLoginTime}
	values := map[string]string{
		userSessionFieldGatewayKey: "gateway-1",
		userSessionFieldLoginTime:  "123456",
	}

	records := buildUserSessionRecordResponse(reqFields, fields, values)
	if len(records) != 2 {
		t.Fatalf("records length = %d, want 2", len(records))
	}
	if records[0].GetField() != pb.CacheUserSessionField_CacheUserSessionField_GatewayKey || records[0].GetValue() != "gateway-1" {
		t.Fatalf("records[0] = %#v", records[0])
	}
	if records[1].GetField() != pb.CacheUserSessionField_CacheUserSessionField_LoginTime || records[1].GetValue() != "123456" {
		t.Fatalf("records[1] = %#v", records[1])
	}
}
