package main

import (
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
)

// OnlineStreamTunnel 接收 gateway 上行帧；注册包绑定 gateway stream；下行由 Gateway.actor 串行发送。
func (p *onlineGRPCServer) OnlineStreamTunnel(stream pb.OnlineService_OnlineStreamTunnelServer) error {
	var gateway *Gateway
	var gatewayKey string
	for {
		req, err := stream.Recv()
		if err != nil {
			if gatewayKey != "" {
				GGatewayMgr.ResetStream(gatewayKey, stream)
			}
			return err
		}
		if req.GetGatewayKey() != "" {
			gatewayKey = req.GetGatewayKey()
			gateway = GGatewayMgr.BindStream(req.GetGatewayKey(), stream)
			xlog.GLog.Infof("OnlineStreamTunnel bind gateway_key:%s", req.GetGatewayKey())
			continue
		}
		for _, frame := range req.GetFrames() {
			if gateway == nil {
				if gatewayKey != "" {
					gateway = GGatewayMgr.Get(gatewayKey)
				}
				if gateway == nil {
					xlog.GLog.Errorf("OnlineStreamTunnel frame before gateway bind aid:%d", frame.GetAid())
					continue
				}
			}
			if pkt := frame.GetClientPacket(); pkt != nil {
				account := GAccountMgr.GetByAID(frame.GetAid())
				if account == nil {
					xlog.GLog.Warnf("OnlineStreamTunnel account not found aid:%d messageID:%d", frame.GetAid(), pkt.GetMessageId())
					continue
				}
				account.PostClientPacket(gateway, pkt)
				continue
			}
			xlog.GLog.Warnf("OnlineStreamTunnel unknown frame payload aid:%d", frame.GetAid())
		}
	}
}
