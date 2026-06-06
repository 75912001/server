package main

import (
	"context"
	"server/common"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// GatewayKickUser 处理新 gateway 发来的顶号请求。
// 调用方必须携带旧连接的 uid 和 userSession；本 gateway 只清理本地仍然匹配该 userSession 的旧连接。
// 返回成功表示旧 TCP、旧 online actor 和 cache session 清理流程已经同步执行完成。
func (p *gatewayGRPCServer) GatewayKickUser(_ context.Context, req *pb.GatewayKickUserReq) (*pb.GatewayKickUserRes, error) {
	userSession := req.GetUserSession()
	if req.GetUid() == 0 || userSession == "" {
		return &pb.GatewayKickUserRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}

	uid := req.GetUid()
	// 顶号只面向已经完成登录验证的用户；未绑定 uid 说明旧连接已不在当前 gateway。
	user := GUserMgr.GetByUID(uid)
	if user == nil {
		return &pb.GatewayKickUserRes{}, grpcstatus.Errorf(grpccodes.NotFound, "not found uid:%d", req.GetUid())
	}
	// 只断开 userSession 匹配的连接，防迟到顶号误踢新连接。
	if user.userSession != userSession {
		return &pb.GatewayKickUserRes{}, grpcstatus.Errorf(grpccodes.Aborted, "user session changed uid:%d", req.GetUid())
	}

	// 设置断开原因后走统一 Remove 路径，确保本地索引、online actor 和 cache session 按同一套清理逻辑处理。
	user.remote.SetDisconnectReason(xnetcommon.DisconnectReason(req.GetReason()))
	if _, err := GUserMgr.Remove(user.remote); err != nil {
		return &pb.GatewayKickUserRes{}, grpcstatus.Errorf(grpccodes.FailedPrecondition, "kick cleanup failed uid:%d err:%v", req.GetUid(), err)
	}

	xlog.GLog.Debugf("phase=kick_user uid=%d userSession=%s reason=%v msg=%s",
		req.GetUid(), common.ShortSession(userSession), xnetcommon.DisconnectReason(req.GetReason()), req.GetMsg())
	return &pb.GatewayKickUserRes{}, nil
}
