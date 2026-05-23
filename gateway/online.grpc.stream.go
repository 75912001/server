package main

import (
	"fmt"

	pb "server/proto/pb"

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
		switch payload := frame.Payload.(type) {
		case *pb.OnlineTunnelFrame_KickUserReq:
			// Online 服要求踢出玩家：找到对应 TCP 连接并主动断开
			// TODO: remote := UserMgr.Get(uid); remote.Close()
			xlog.PrintInfo(fmt.Sprintf("kick uid=%d reason=%d msg=%s",
				uid, payload.KickUserReq.GetReason(), payload.KickUserReq.GetMsg()))
		case *pb.OnlineTunnelFrame_ClientPacket:
			// 业务下行包：原封不动转发给客户端
			// TODO: remote := UserMgr.Get(uid); remote.Send(...)
			pkt := payload.ClientPacket
			xlog.PrintInfo(fmt.Sprintf("downstream uid=%d messageID=%d len=%d",
				uid, pkt.GetMessageId(), len(pkt.GetBody())))
		default:
			xlog.PrintfErr("online stream: unexpected frame payload type for uid=%d", uid)
		}
	}
	return nil
}

// OnlineStreamTunnelPost 流关闭时触发：遍历 GOnlineMgr 找到持有该 stream 的实例并置空。
func (h *onlineStreamHandler) OnlineStreamTunnelPost(stream pb.OnlineService_OnlineStreamTunnelClient) error {
	GOnlineMgr.m.Foreach(func(_ string, o *Online) bool {
		o.streamMu.RLock()
		match := o.GetStream() == stream
		o.streamMu.RUnlock()
		if match {
			o.resetStream()
			return false // 找到即停
		}
		return true
	})
	return nil
}
