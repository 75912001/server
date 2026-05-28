package main

import (
	"context"
	pb "server/proto/pb"

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
