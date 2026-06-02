package main

import (
	"context"
	"fmt"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *User) onLogin(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	uid := req.GetUid()
	currentOnlineKey := xetcd.GEtcd.GetKey()
	oldSession, err := unaryCacheGetUserSession(uid)
	if err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	hasOldSession := oldSession.gatewayKey != "" || oldSession.onlineKey != ""
	if !hasOldSession && p.gatewayID != "" {
		oldSession.gatewayKey = p.gatewayID
		oldSession.onlineKey = currentOnlineKey
		hasOldSession = true
	}
	if hasOldSession && oldSession.gatewayKey != "" {
		removed := false
		if currentUser, ok := GUserMgr.users.Find(uid); ok && currentUser == p {
			GUserMgr.users.Del(uid)
			removed = true
		}
		if err := p.kickGateway(oldSession.gatewayKey); err != nil {
			if removed {
				GUserMgr.users.Add(uid, p)
			}
			return nil, grpcstatus.Error(grpccodes.FailedPrecondition, fmt.Sprintf("kick old gateway failed: %v", err))
		}
	}
	sessionCommitted := false
	if err := unaryCacheSetUserSession(uid, req.GetGatewayKey(), currentOnlineKey); err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	sessionCommitted = true
	userRecord, err := unaryCacheGetUserRecord(req.GetUid())
	if err != nil {
		if cleanupErr := p.cleanupCommittedUserSession(uid, sessionCommitted); cleanupErr != nil {
			return nil, grpcstatus.Error(grpccodes.Internal, fmt.Sprintf("%v; cleanup user session failed: %v", err, cleanupErr))
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	p.gatewayID = req.GetGatewayKey()
	p.clientIP = req.GetClientIp()
	p.userRecord = userRecord.GetUserRecord()
	GUserMgr.users.Add(uid, p)
	return &pb.OnlineUserOnlineRes{}, nil
}

func (p *User) cleanupCommittedUserSession(uid uint64, sessionCommitted bool) error {
	if !sessionCommitted {
		return nil
	}
	return unaryCacheDelUserSession(uid)
}

func (p *User) kickGateway(gatewayKey string) error {
	gateway := GGatewayMgr.Get(gatewayKey)
	if gateway == nil {
		return nil
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
		if s, ok := grpcstatus.FromError(err); ok {
			if s.Code() == grpccodes.NotFound {
				return nil
			}
		}
		return err
	}
	return nil
}
