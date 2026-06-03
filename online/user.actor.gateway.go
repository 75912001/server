package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcproto "github.com/75912001/xlib/grpc/proto"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *User) onLogin(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	uid := p.uid
	account := strings.TrimSpace(req.GetAccount())
	if uid == 0 || account == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	currentOnlineKey := xetcd.GEtcd.GetKey()
	oldSession, err := unaryCacheGetUserSession(uid)
	if err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	newSession, err := newCacheUserSession(req.GetGatewayKey(), currentOnlineKey, req.GetGatewaySession())
	if err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if cacheUserSessionIsEmpty(oldSession) && p.gatewayID != "" {
		oldSession = cloneCacheUserSession(p.sessionMgr.session)
		if oldSession.gatewayKey == "" {
			oldSession.gatewayKey = p.gatewayID
		}
		if oldSession.onlineKey == "" {
			oldSession.onlineKey = currentOnlineKey
		}
	}
	if !cacheUserSessionIsEmpty(oldSession) && oldSession.gatewayKey != "" {
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
	expectedSession, err := expectedCacheUserSessionAfterKick(uid, oldSession)
	if err != nil {
		return nil, err
	}
	recordRes, err := unaryCacheGetUserRecord(uid)
	if err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	userRecord := recordRes.GetUserRecord()
	if userRecord == nil || userRecord.GetUid() != uid || userRecord.GetAccount() != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "user record mismatch")
	}
	if err := unaryCacheReplaceUserSession(uid, expectedSession, newSession); err != nil {
		if s, ok := grpcstatus.FromError(err); ok && s.Code() == grpccodes.Aborted {
			return nil, grpcstatus.Error(grpccodes.Aborted, err.Error())
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	p.gatewayID = req.GetGatewayKey()
	p.account = userRecord.GetAccount()
	p.clientIP = req.GetClientIp()
	p.sessionMgr.Bind(newSession)
	p.userRecord = userRecord
	GUserMgr.users.Add(uid, p)
	return &pb.OnlineUserOnlineRes{}, nil
}

func (p *User) onUpdateGatewaySession(gatewayKey string, oldGatewaySession string, newGatewaySession string) error {
	if gatewayKey == "" || oldGatewaySession == "" || newGatewaySession == "" {
		return grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if p.gatewayID != gatewayKey || p.sessionMgr.session == nil {
		return grpcstatus.Error(grpccodes.NotFound, "user gatewaySession not found")
	}
	if p.sessionMgr.session.gatewaySession != oldGatewaySession {
		return grpcstatus.Error(grpccodes.Aborted, "user gatewaySession changed")
	}
	if err := p.sessionMgr.UpdateGatewaySession(newGatewaySession); err != nil {
		return err
	}
	return nil
}

func cloneCacheUserSession(session *cacheUserSession) *cacheUserSession {
	if session == nil {
		return &cacheUserSession{}
	}
	return &cacheUserSession{
		gatewayKey:     session.gatewayKey,
		onlineKey:      session.onlineKey,
		gatewaySession: session.gatewaySession,
		loginTime:      session.loginTime,
	}
}

func cacheUserSessionIsEmpty(session *cacheUserSession) bool {
	return session == nil || (session.gatewayKey == "" && session.onlineKey == "" && session.gatewaySession == "")
}

func cacheUserSessionMatch(a *cacheUserSession, b *cacheUserSession) bool {
	if a == nil || b == nil {
		return cacheUserSessionIsEmpty(a) && cacheUserSessionIsEmpty(b)
	}
	return a.gatewayKey == b.gatewayKey && a.onlineKey == b.onlineKey && a.gatewaySession == b.gatewaySession
}

func expectedCacheUserSessionAfterKick(uid uint64, oldSession *cacheUserSession) (*cacheUserSession, error) {
	currentSession, err := unaryCacheGetUserSession(uid)
	if err != nil {
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if cacheUserSessionIsEmpty(currentSession) {
		return &cacheUserSession{}, nil
	}
	if cacheUserSessionMatch(currentSession, oldSession) {
		return oldSession, nil
	}
	return nil, grpcstatus.Error(grpccodes.Aborted, fmt.Sprintf("user gatewaySession changed uid:%d", uid))
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
	ctx := xgrpcproto.SetFromOutgoingContext(context.Background(), xgrpcproto.ShardKeyFieldNameDefault, strconv.FormatUint(p.uid, 10))
	_, err = client.GatewayUserOffline(ctx, &pb.GatewayUserOfflineReq{
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
