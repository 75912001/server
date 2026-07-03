package main

import (
	"context"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// GatewayKickAccountSession 处理新 gateway 发来的顶号请求。
// 调用方必须携带旧连接的 aid 和 accountSession；本 gateway 只清理本地仍然匹配该 accountSession 的旧连接。
// 返回成功表示旧 TCP、旧 online actor 和 cache session 清理流程已经同步执行完成。
func (p *gatewayGRPCServer) GatewayKickAccountSession(_ context.Context, req *pb.GatewayKickAccountSessionReq) (*pb.GatewayKickAccountSessionRes, error) {
	accountSession := req.GetAccountSession()
	if req.GetAid() == 0 || accountSession == "" {
		return &pb.GatewayKickAccountSessionRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}

	aid := req.GetAid()
	// 顶号只面向已经完成登录验证的用户；未绑定 aid 说明旧连接已不在当前 gateway。
	account := GAccountMgr.GetByAID(aid)
	if account == nil {
		return &pb.GatewayKickAccountSessionRes{}, grpcstatus.Errorf(grpccodes.NotFound, "not found aid:%d", req.GetAid())
	}
	// 只断开 accountSession 匹配的连接，防迟到顶号误踢新连接。
	if account.accountSession != accountSession {
		return &pb.GatewayKickAccountSessionRes{}, grpcstatus.Errorf(grpccodes.Aborted, "account session changed aid:%d", req.GetAid())
	}

	// 设置断开原因后走统一 Remove 路径，确保本地索引、online actor 和 cache session 按同一套清理逻辑处理。
	account.remote.SetDisconnectReason(xnetcommon.DisconnectReason(req.GetReason()))
	if _, err := GAccountMgr.Remove(account.remote); err != nil {
		return &pb.GatewayKickAccountSessionRes{}, grpcstatus.Errorf(grpccodes.FailedPrecondition, "kick cleanup failed aid:%d err:%v", req.GetAid(), err)
	}

	xlog.GLog.Debugf("phase=kick_account aid=%d accountSession=%s reason=%v msg=%s",
		req.GetAid(), accountSession, xnetcommon.DisconnectReason(req.GetReason()), req.GetMsg())
	return &pb.GatewayKickAccountSessionRes{}, nil
}
