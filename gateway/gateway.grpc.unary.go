package main

import (
	"context"
	"fmt"

	common "server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
)

func (p *gatewayGRPCServer) GatewayUserOffline(_ context.Context, req *pb.GatewayUserOfflineReq) (*pb.GatewayUserOfflineRes, error) {
	if req.GetUid() == 0 {
		xlog.GLog.Error("GatewayUserOffline uid:0")
		return &pb.GatewayUserOfflineRes{
			Code: common.ECGatewayInvalidUID.Code(),
			Msg:  common.ECGatewayInvalidUID.Error(),
		}, nil
	}

	user := GUserMgr.GetByUID(req.GetUid())
	if user == nil {
		xlog.GLog.Warnf("not found uid:%d", req.GetUid())
		return &pb.GatewayUserOfflineRes{
			Code: xerror.Success.Code(),
			Msg:  common.ECGatewayUIDNotFound.Error(),
		}, nil
	}

	user.Disconnect(xnetcommon.DisconnectReason(req.GetReason()))

	xlog.GLog.Debugf("GatewayUserOffline uid:%d reason:%v msg:%s", req.GetUid(), xnetcommon.DisconnectReason(req.GetReason()), req.GetMsg())
	return &pb.GatewayUserOfflineRes{
		Code: xerror.Success.Code(),
		Msg:  fmt.Sprintf("uid:%d offline", req.GetUid()),
	}, nil
}
