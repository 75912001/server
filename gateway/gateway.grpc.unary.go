package main

import (
	"context"
	"time"

	"server/common"
	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *gatewayGRPCServer) GatewayUserOffline(_ context.Context, req *pb.GatewayUserOfflineReq) (*pb.GatewayUserOfflineRes, error) {
	if req.GetUid() == 0 {
		return &pb.GatewayUserOfflineRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid uid:0")
	}

	uid := req.GetUid()
	user := GUserMgr.GetByUID(uid)
	if user == nil {
		return &pb.GatewayUserOfflineRes{}, grpcstatus.Errorf(grpccodes.NotFound, "not found uid:%d", req.GetUid())
	}

	user.Disconnect(xnetcommon.DisconnectReason(req.GetReason()))

	xlog.GLog.Debugf("GatewayUserOffline uid:%d reason:%v msg:%s", req.GetUid(), xnetcommon.DisconnectReason(req.GetReason()), req.GetMsg())
	return &pb.GatewayUserOfflineRes{}, nil
}

func (p *gatewayGRPCServer) GatewayPrepareLogin(_ context.Context, req *pb.GatewayPrepareLoginReq) (*pb.GatewayPrepareLoginRes, error) {
	uid := req.GetUid()
	account := req.GetAccount()
	gatewayNonce := req.GetGatewayNonce()
	gatewaySession := req.GetGatewaySession()
	expireSecond := req.GetExpireSecond()
	if uid == 0 || account == "" || gatewayNonce == "" || gatewaySession == "" || expireSecond == 0 {
		return &pb.GatewayPrepareLoginRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if common.NewGatewaySession(uid, xetcd.GEtcd.GetKey(), gatewayNonce) != gatewaySession {
		return &pb.GatewayPrepareLoginRes{}, grpcstatus.Error(grpccodes.Unauthenticated, "invalid gateway session")
	}
	GLoginSessionMgr.Add(uid, account, gatewayNonce, gatewaySession, time.Duration(expireSecond)*time.Second)
	xlog.GLog.Debugf("GatewayPrepareLogin uid:%d account:%s expireSecond:%d", uid, account, expireSecond)
	return &pb.GatewayPrepareLoginRes{}, nil
}
