package main

import (
	"context"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineUserOnline(_ context.Context, req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	uid := req.GetUid()
	token := req.GetToken()
	if err := unaryCacheVerifyUserToken(uid, token); err != nil {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, err.Error())
	}
	res, err := GUserMgr.Login(req)
	if err != nil {
		return nil, err
	}
	if res == nil || res.GetSession() == "" {
		return nil, grpcstatus.Error(grpccodes.Internal, "online login session is empty")
	}
	if err := unaryCacheUseVerifyUserToken(uid, token); err != nil {
		cleanupOnlineLogin(req, res.GetSession())
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, err.Error())
	}
	return res, nil
}

func (p *onlineGRPCServer) OnlineUserOffline(_ context.Context, req *pb.OnlineUserOfflineReq) (*pb.OnlineUserOfflineRes, error) {
	if req.GetUid() == 0 || req.GetGatewayKey() == "" || req.GetSession() == "" {
		return &pb.OnlineUserOfflineRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	user, ok := GUserMgr.users.Find(req.GetUid())
	if !ok {
		return &pb.OnlineUserOfflineRes{}, nil
	}
	user.PostOffline(req.GetGatewayKey(), req.GetSession())
	return &pb.OnlineUserOfflineRes{}, nil
}

func cleanupOnlineLogin(req *pb.OnlineUserOnlineReq, session string) {
	if req.GetUid() == 0 || req.GetGatewayKey() == "" || session == "" {
		return
	}
	user, ok := GUserMgr.users.Find(req.GetUid())
	if !ok {
		return
	}
	user.PostOffline(req.GetGatewayKey(), session)
}
