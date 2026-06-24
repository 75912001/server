package main

import (
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *Account) onBind(req *pb.OnlineBindUserReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindUserRes, error) {
	uid := p.uid
	account := strings.TrimSpace(req.GetAccount())
	gatewayKey := strings.TrimSpace(req.GetGatewayKey())
	userSession := req.GetUserSession()
	if uid == 0 || account == "" || gatewayKey == "" || userSession == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if accountRecord == nil || accountRecord.GetUid() != uid || strings.TrimSpace(accountRecord.GetAccount()) != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "account record mismatch")
	}
	if accountRecord.GetAccountCreateTimestampMs() == 0 {
		return nil, grpcstatus.Error(grpccodes.Internal, "invalid account record")
	}

	p.gatewayKey = gatewayKey
	p.userSession = userSession
	p.account = accountRecord.GetAccount()
	p.clientIP = req.GetClientIp()
	p.accountRecord = accountRecord
	GAccountMgr.accounts.Add(uid, p)
	return &pb.OnlineBindUserRes{}, nil
}
