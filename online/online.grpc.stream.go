package main

import (
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
)

// OnlineStreamTunnel 接收 gateway 上行帧；注册包绑定 gateway stream；下行由 Gateway.actor 串行发送。
func (p *onlineGRPCServer) OnlineStreamTunnel(stream pb.OnlineService_OnlineStreamTunnelServer) error {
	var gateway *Gateway
	var gatewayID string
	for {
		req, err := stream.Recv()
		if err != nil {
			if gatewayID != "" {
				GGatewayMgr.ResetStream(gatewayID, stream)
			}
			return err
		}
		if req.GetGatewayId() != "" {
			gatewayID = req.GetGatewayId()
			gateway = GGatewayMgr.BindStream(req.GetGatewayId(), stream)
			xlog.GLog.Infof("OnlineStreamTunnel bind gateway_id:%s", req.GetGatewayId())
			continue
		}
		for _, frame := range req.GetFrames() {
			if gateway == nil {
				if gatewayID != "" {
					gateway = GGatewayMgr.Get(gatewayID)
				}
				if gateway == nil {
					xlog.GLog.Errorf("OnlineStreamTunnel frame before gateway bind uid:%d", frame.GetUid())
					continue
				}
			}
			if pkt := frame.GetClientPacket(); pkt != nil {
				user := GUserMgr.GetByUID(frame.GetUid())
				if user == nil {
					xlog.GLog.Warnf("OnlineStreamTunnel user not found uid:%d messageID:%d", frame.GetUid(), pkt.GetMessageId())
					continue
				}
				user.PostClientPacket(gateway, pkt)
				continue
			}
			xlog.GLog.Warnf("OnlineStreamTunnel unknown frame payload uid:%d", frame.GetUid())
		}
	}
}
