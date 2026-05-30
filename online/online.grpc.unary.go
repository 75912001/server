package main

import (
	"context"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineUserOnline(_ context.Context, req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	if _, err := unaryCacheVerifyUserToken(req.GetUid(), req.GetToken()); err != nil {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, err.Error())
	}
	return GUserMgr.Login(req)
}

func (p *onlineGRPCServer) OnlineUserOffline(_ context.Context, req *pb.OnlineUserOfflineReq) (*pb.OnlineUserOfflineRes, error) {
	if req.GetUid() == 0 {
		return &pb.OnlineUserOfflineRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "uid is empty")
	}
	user, ok := GUserMgr.Get(req.GetUid())
	if !ok {
		return &pb.OnlineUserOfflineRes{}, grpcstatus.Error(grpccodes.NotFound, "user not found")
	}
	user.PostOffline()
	return &pb.OnlineUserOfflineRes{}, nil
}
