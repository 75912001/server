package main

import (
	"context"
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineBindUser(_ context.Context, req *pb.OnlineBindUserReq) (*pb.OnlineBindUserRes, error) {
	uid := req.GetUid()
	account := strings.TrimSpace(req.GetAccount())
	if uid == 0 || account == "" || req.GetGatewayKey() == "" || req.GetUserSession() == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	userRecord, err := unaryCacheGetUserRecord(uid)
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, grpcstatus.Error(s.Code(), s.Message())
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if userRecord == nil || userRecord.GetUid() != uid || strings.TrimSpace(userRecord.GetAccount()) != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "user record mismatch")
	}
	req.Account = account
	res, err := GUserMgr.Bind(uid, req, userRecord)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, grpcstatus.Error(grpccodes.Internal, "online bind response is empty")
	}
	return res, nil
}

func (p *onlineGRPCServer) OnlineUnbindUser(_ context.Context, req *pb.OnlineUnbindUserReq) (*pb.OnlineUnbindUserRes, error) {
	if req.GetUid() == 0 || req.GetGatewayKey() == "" || req.GetUserSession() == "" {
		return &pb.OnlineUnbindUserRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	user, ok := GUserMgr.users.Find(req.GetUid())
	if !ok {
		return &pb.OnlineUnbindUserRes{}, nil
	}
	user.PostUnbind(req.GetGatewayKey(), req.GetUserSession())
	return &pb.OnlineUnbindUserRes{}, nil
}
