package main

import (
	"context"
	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineBindAccount(_ context.Context, req *pb.OnlineBindAccountReq) (*pb.OnlineBindAccountRes, error) {
	aid := req.GetAid()
	account := req.GetAccount()
	if aid == 0 || account == "" || req.GetGatewayKey() == "" || req.GetAccountSession() == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	accountRecord, err := unaryCacheGetAccountRecord(aid)
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, grpcstatus.Error(s.Code(), s.Message())
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if accountRecord == nil || accountRecord.GetAid() != aid || accountRecord.GetAccount() != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "account record mismatch")
	}
	if accountRecord.GetAccountCreateTimestampMs() == 0 {
		return nil, grpcstatus.Error(grpccodes.Internal, "invalid account record")
	}
	req.Account = account
	res, err := GAccountMgr.Bind(aid, req, accountRecord)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, grpcstatus.Error(grpccodes.Internal, "online bind response is empty")
	}
	return res, nil
}

func (p *onlineGRPCServer) OnlineUnbindAccount(_ context.Context, req *pb.OnlineUnbindAccountReq) (*pb.OnlineUnbindAccountRes, error) {
	if req.GetAid() == 0 || req.GetGatewayKey() == "" || req.GetAccountSession() == "" {
		return &pb.OnlineUnbindAccountRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	account, ok := GAccountMgr.accounts.Find(req.GetAid())
	if !ok {
		return &pb.OnlineUnbindAccountRes{}, nil
	}
	account.PostUnbind(req.GetGatewayKey(), req.GetAccountSession())
	return &pb.OnlineUnbindAccountRes{}, nil
}
