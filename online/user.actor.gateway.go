package main

import (
	"context"
	"fmt"
	"strconv"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcproto "github.com/75912001/xlib/grpc/proto"
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
	newSession, err := newCacheUserSession(req.GetGatewayKey(), currentOnlineKey)
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
	userRecord, err := unaryCacheGetUserRecord(req.GetUid())
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok && s.Code() == grpccodes.NotFound {
			userRecord = &pb.CacheGetUserRecordRes{UserRecord: &pb.UserRecord{}}
		} else {
			return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
		}
	}
	expectedSession, err := expectedCacheUserSessionAfterKick(uid, oldSession)
	if err != nil {
		return nil, err
	}
	if err := unaryCacheReplaceUserSession(uid, expectedSession, newSession); err != nil {
		if s, ok := grpcstatus.FromError(err); ok && s.Code() == grpccodes.Aborted {
			return nil, grpcstatus.Error(grpccodes.Aborted, err.Error())
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	p.gatewayID = req.GetGatewayKey()
	p.clientIP = req.GetClientIp()
	p.sessionMgr.Bind(newSession)
	p.userRecord = userRecord.GetUserRecord()
	GUserMgr.users.Add(uid, p)
	return &pb.OnlineUserOnlineRes{Session: newSession.session}, nil
}

func cloneCacheUserSession(session *cacheUserSession) *cacheUserSession {
	if session == nil {
		return &cacheUserSession{}
	}
	return &cacheUserSession{
		gatewayKey: session.gatewayKey,
		onlineKey:  session.onlineKey,
		session:    session.session,
		loginTime:  session.loginTime,
	}
}

func cacheUserSessionIsEmpty(session *cacheUserSession) bool {
	return session == nil || (session.gatewayKey == "" && session.onlineKey == "" && session.session == "")
}

func cacheUserSessionMatch(a *cacheUserSession, b *cacheUserSession) bool {
	if a == nil || b == nil {
		return cacheUserSessionIsEmpty(a) && cacheUserSessionIsEmpty(b)
	}
	return a.gatewayKey == b.gatewayKey && a.onlineKey == b.onlineKey && a.session == b.session
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
	return nil, grpcstatus.Error(grpccodes.Aborted, fmt.Sprintf("user session changed uid:%d", uid))
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
