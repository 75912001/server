package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
)

// GOnlineStreamHandler 全局流回调，init 时注册到 pb 包。
var GOnlineStreamHandler = &onlineStreamHandler{}

func init() {
	pb.SetIStreamOnlineServiceOnlineStreamTunnelClient(GOnlineStreamHandler)
}

type onlineStreamHandler struct{}

// OnlineStreamTunnelPre 流建立时触发（流已在 NewXOnlineService 中创建，此处无需额外处理）
func (h *onlineStreamHandler) OnlineStreamTunnelPre(_ pb.OnlineService_OnlineStreamTunnelClient) error {
	return nil
}

// OnlineStreamTunnel 处理 online → gateway 下行流。
// 每条 OnlineStreamTunnelRes 含若干 OnlineTunnelFrame，按 payload 类型分发：
func (h *onlineStreamHandler) OnlineStreamTunnel(
	_ *pb.XStreamOnlineServiceOnlineStreamTunnelClient,
	msg *pb.OnlineStreamTunnelRes,
	_ pb.OnlineService_OnlineStreamTunnelClient,
) error {
	for _, frame := range msg.GetFrames() {
		uid := frame.GetUid()
		GUserMgr.PostOnlineFrame(uid, frame)
	}
	return nil
}

// OnlineStreamTunnelPost 流关闭时触发：向所有 Online 的 actor 投递 OnlineActorCmdStreamReset，
// 由各 actor 自行判断是否是自己的流，匹配则置 nil。
func (h *onlineStreamHandler) OnlineStreamTunnelPost(stream pb.OnlineService_OnlineStreamTunnelClient) error {
	GOnlineMgr.m.Foreach(func(_ string, o *Online) bool {
		o.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineActorCmdStreamReset, stream))
		return true
	})
	return nil
}
