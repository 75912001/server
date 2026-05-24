package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
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
//
//	kick_user_req  → 找到 uid 对应的 TCP 连接并断开（踢人）
//	client_packet  → 找到 uid 对应的 TCP 连接并转发（下行业务包）
//
// TODO: 依赖 UserMgr（uid → IRemote），后续实现用户会话管理后补全
func (h *onlineStreamHandler) OnlineStreamTunnel(
	_ *pb.XStreamOnlineServiceOnlineStreamTunnelClient,
	msg *pb.OnlineStreamTunnelRes,
	_ pb.OnlineService_OnlineStreamTunnelClient,
) error {
	for _, frame := range msg.GetFrames() {
		uid := frame.GetUid()
		switch frame.Payload.(type) {
		case *pb.OnlineTunnelFrame_KickUserReq:
			GUserMgr.PostOnlineFrame(uid, frame)
		case *pb.OnlineTunnelFrame_ClientPacket:
			GUserMgr.PostOnlineFrame(uid, frame)
		default:
			xlog.PrintfErr("online stream: unexpected frame payload type for uid=%d", uid)
		}
	}
	return nil
}

// OnlineStreamTunnelPost 流关闭时触发：向所有 Online 的 actor 投递 CmdStreamReset，
// 由各 actor 自行判断是否是自己的流，匹配则置 nil。
func (h *onlineStreamHandler) OnlineStreamTunnelPost(stream pb.OnlineService_OnlineStreamTunnelClient) error {
	GOnlineMgr.m.Foreach(func(_ string, o *Online) bool {
		o.actor.SendMsg(xactor.NewMsg(context.Background(), CmdStreamReset, stream))
		return true
	})
	return nil
}
