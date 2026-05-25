package main

import (
	"context"
	"fmt"

	common "server/common"
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
)

type gatewayGRPCServer struct {
	pb.UnimplementedGatewayServiceServer
}

func (s *gatewayGRPCServer) GatewayKickUser(_ context.Context, req *pb.GatewayKickUserReq) (*pb.GatewayKickUserRes, error) {
	if req.GetUid() == 0 {
		return &pb.GatewayKickUserRes{
			Code: common.ECGatewayInvalidUID.Code(),
			Msg:  common.ECGatewayInvalidUID.Error(),
		}, nil
	}
	kicked := GUserMgr.KickVerifiedUID(req.GetUid(), req.GetReason(), req.GetMsg())
	if !kicked {
		return &pb.GatewayKickUserRes{
			Code: 0,
			Msg:  fmt.Sprintf("uid=%d already offline", req.GetUid()),
		}, nil
	}
	xlog.PrintInfo(fmt.Sprintf("GatewayKickUser uid=%d reason=%d msg=%s", req.GetUid(), req.GetReason(), req.GetMsg()))
	return &pb.GatewayKickUserRes{
		Code: 0,
		Msg:  fmt.Sprintf("uid=%d kicked", req.GetUid()),
	}, nil
}
