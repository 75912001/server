package main

import (
	"context"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
)

// GOnlineStreamHandler 全局流回调，init 时注册到 pb 包。
var GOnlineStreamHandler = &onlineStreamHandler{}

func init() {
	pb.SetIStreamOnlineServiceOnlineStreamTunnelClient(GOnlineStreamHandler)
}

type onlineStreamHandler struct{}

// OnlineStreamTunnelPre 流建立时发送 gateway_id 注册包，让 online 绑定该 gateway stream。
func (p *onlineStreamHandler) OnlineStreamTunnelPre(stream pb.OnlineService_OnlineStreamTunnelClient) error {
	return stream.Send(&pb.OnlineStreamTunnelReq{GatewayId: xetcd.GEtcd.GetKey()})
}

// OnlineStreamTunnel 处理 online → gateway 下行流。
// 每条 OnlineStreamTunnelRes 含若干 OnlineTunnelFrame，按 uid 投递到对应用户 actor。
func (p *onlineStreamHandler) OnlineStreamTunnel(
	_ *pb.XStreamOnlineServiceOnlineStreamTunnelClient,
	msg *pb.OnlineStreamTunnelRes,
	_ pb.OnlineService_OnlineStreamTunnelClient,
) error {
	for _, frame := range msg.GetFrames() {
		uid := frame.GetUid()
		user := GUserMgr.GetByUID(uid)
		if user == nil {
			xlog.GLog.Warnf("online frame uid:%d not found", uid)
			continue
		}
		user.PostFrame(frame)
	}
	return nil
}

// OnlineStreamTunnelPost 流关闭时触发：向所有 Online 的 actor 投递 OnlineActorCmdStreamReset，
// 由各 actor 自行判断是否是自己的流，匹配则置 nil。
func (p *onlineStreamHandler) OnlineStreamTunnelPost(stream pb.OnlineService_OnlineStreamTunnelClient) error {
	GOnlineMgr.m.Foreach(func(_ string, o *Online) bool {
		o.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineActorCmdStreamReset, stream))
		return true
	})
	return nil
}
