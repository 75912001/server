package main

import (
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *User) onBind(req *pb.OnlineBindUserReq, userRecord *pb.UserRecord) (*pb.OnlineBindUserRes, error) {
	uid := p.uid
	account := strings.TrimSpace(req.GetAccount())
	gatewayKey := strings.TrimSpace(req.GetGatewayKey())
	userSession := req.GetUserSession()
	if uid == 0 || account == "" || gatewayKey == "" || userSession == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if userRecord == nil || userRecord.GetUid() != uid || strings.TrimSpace(userRecord.GetAccount()) != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "user record mismatch")
	}
	if userRecord.GetAccountCreateTimeMs() == 0 {
		return nil, grpcstatus.Error(grpccodes.Internal, "invalid user record")
	}

	p.gatewayID = gatewayKey
	p.userSession = userSession
	p.account = userRecord.GetAccount()
	p.clientIP = req.GetClientIp()
	p.userRecord = userRecord
	GUserMgr.users.Add(uid, p)
	return &pb.OnlineBindUserRes{}, nil
}
