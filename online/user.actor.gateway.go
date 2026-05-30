package main

import (
	"context"
	"fmt"

	pb "server/proto/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (p *User) onLogin(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	userRecord, err := unaryCacheGetUserRecord(req.GetUid())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if userRecord.GetUserRecord() == nil {
		return nil, status.Error(codes.Internal, "user record is nil")
	}
	p.gatewayID = req.GetGatewayKey()
	p.clientIP = req.GetClientIp()
	p.userRecord = userRecord.GetUserRecord()
	return &pb.OnlineUserOnlineRes{}, nil
}

func (p *User) kickOldGateway() error {
	gateway := GGatewayMgr.Get(p.gatewayID)
	if gateway == nil {
		return fmt.Errorf("old gateway %s not found", p.gatewayID)
	}
	client, err := gateway.Client()
	if err != nil {
		return err
	}
	_, err = client.GatewayUserOffline(context.Background(), &pb.GatewayUserOfflineReq{
		Uid:    p.uid,
		Reason: 1,
		Msg:    "duplicate login",
	})
	if err != nil {
		return err
	}
	return nil
}
