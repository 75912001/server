package main

import (
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *Account) onBind(req *pb.OnlineBindAccountReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindAccountRes, error) {
	aid := p.aid
	account := strings.TrimSpace(req.GetAccount())
	gatewayKey := strings.TrimSpace(req.GetGatewayKey())
	accountSession := req.GetAccountSession()
	if aid == 0 || account == "" || gatewayKey == "" || accountSession == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if accountRecord == nil || accountRecord.GetAid() != aid || strings.TrimSpace(accountRecord.GetAccount()) != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "account record mismatch")
	}
	if accountRecord.GetAccountCreateTimestampMs() == 0 {
		return nil, grpcstatus.Error(grpccodes.Internal, "invalid account record")
	}

	p.gatewayKey = gatewayKey
	p.accountSession = accountSession
	p.account = accountRecord.GetAccount()
	p.clientIP = req.GetClientIp()
	p.accountRecord = accountRecord
	p.clearOnlineCharacterUUIDs()
	GAccountMgr.accounts.Add(aid, p)
	return &pb.OnlineBindAccountRes{}, nil
}
