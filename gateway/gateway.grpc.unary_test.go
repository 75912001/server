package main

import (
	"context"
	"testing"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
)

func TestGatewayKickUserValidation(t *testing.T) {
	resetTestGatewayUserMgr(t)

	srv := &gatewayGRPCServer{}

	tests := []struct {
		name string
		req  *pb.GatewayKickUserReq
		want grpccodes.Code
	}{
		{
			name: "missing uid",
			req: &pb.GatewayKickUserReq{
				UserSession: "session-1",
			},
			want: grpccodes.InvalidArgument,
		},
		{
			name: "missing user session",
			req: &pb.GatewayKickUserReq{
				Uid: 1001,
			},
			want: grpccodes.InvalidArgument,
		},
		{
			name: "user not found",
			req: &pb.GatewayKickUserReq{
				Uid:         1001,
				UserSession: "session-1",
			},
			want: grpccodes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.GatewayKickUser(context.Background(), tt.req)
			requireStatusCode(t, err, tt.want)
		})
	}
}

func TestGatewayKickUserRejectsChangedUserSession(t *testing.T) {
	resetTestGatewayUserMgr(t)

	const uid uint64 = 1001
	GUserMgr.byUID.Add(uid, &User{uid: uid, userSession: "new-session"})

	_, err := (&gatewayGRPCServer{}).GatewayKickUser(context.Background(), &pb.GatewayKickUserReq{
		Uid:         uid,
		UserSession: "old-session",
		Reason:      101,
	})

	requireStatusCode(t, err, grpccodes.Aborted)
}
