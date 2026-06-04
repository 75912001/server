package main

import (
	"context"
	"testing"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
)

func TestGatewayUserOfflineValidation(t *testing.T) {
	resetTestGatewayUserMgr(t)

	srv := &gatewayGRPCServer{}

	tests := []struct {
		name string
		req  *pb.GatewayUserOfflineReq
		want grpccodes.Code
	}{
		{
			name: "missing uid",
			req: &pb.GatewayUserOfflineReq{
				UserSession: "session-1",
			},
			want: grpccodes.InvalidArgument,
		},
		{
			name: "missing user session",
			req: &pb.GatewayUserOfflineReq{
				Uid: 1001,
			},
			want: grpccodes.InvalidArgument,
		},
		{
			name: "user not found",
			req: &pb.GatewayUserOfflineReq{
				Uid:         1001,
				UserSession: "session-1",
			},
			want: grpccodes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.GatewayUserOffline(context.Background(), tt.req)
			requireStatusCode(t, err, tt.want)
		})
	}
}

func TestGatewayUserOfflineRejectsChangedUserSession(t *testing.T) {
	resetTestGatewayUserMgr(t)

	const uid uint64 = 1001
	GUserMgr.byUID.Add(uid, &User{uid: uid, userSession: "new-session"})

	_, err := (&gatewayGRPCServer{}).GatewayUserOffline(context.Background(), &pb.GatewayUserOfflineReq{
		Uid:         uid,
		UserSession: "old-session",
		Reason:      101,
	})

	requireStatusCode(t, err, grpccodes.Aborted)
}
