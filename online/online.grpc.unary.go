package main

import (
	"context"
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineUserOnline(_ context.Context, req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	uid := req.GetUid()
	account := strings.TrimSpace(req.GetAccount())
	gatewaySession := req.GetGatewaySession()
	if uid == 0 || account == "" || req.GetGatewayKey() == "" || gatewaySession == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	req.Account = account
	res, err := GUserMgr.Login(uid, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, grpcstatus.Error(grpccodes.Internal, "online login response is empty")
	}
	return res, nil
}

func (p *onlineGRPCServer) OnlineUserOffline(_ context.Context, req *pb.OnlineUserOfflineReq) (*pb.OnlineUserOfflineRes, error) {
	if req.GetUid() == 0 || req.GetGatewayKey() == "" || req.GetGatewaySession() == "" {
		return &pb.OnlineUserOfflineRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	user, ok := GUserMgr.users.Find(req.GetUid())
	if !ok {
		return &pb.OnlineUserOfflineRes{}, nil
	}
	user.PostOffline(req.GetGatewayKey(), req.GetGatewaySession())
	return &pb.OnlineUserOfflineRes{}, nil
}

func (p *onlineGRPCServer) OnlineUserUpdateGatewaySession(_ context.Context, req *pb.OnlineUserUpdateGatewaySessionReq) (*pb.OnlineUserUpdateGatewaySessionRes, error) {
	uid := req.GetUid()
	if uid == 0 || req.GetGatewayKey() == "" || req.GetOldGatewaySession() == "" || req.GetNewGatewaySession() == "" {
		return &pb.OnlineUserUpdateGatewaySessionRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	user, ok := GUserMgr.users.Find(uid)
	if !ok {
		return &pb.OnlineUserUpdateGatewaySessionRes{}, grpcstatus.Error(grpccodes.NotFound, "user not online")
	}
	if err := user.PostUpdateGatewaySession(req.GetGatewayKey(), req.GetOldGatewaySession(), req.GetNewGatewaySession()); err != nil {
		return &pb.OnlineUserUpdateGatewaySessionRes{}, err
	}
	return &pb.OnlineUserUpdateGatewaySessionRes{}, nil
}
