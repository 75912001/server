package main

import (
	pb "server/proto/pb"

	"context"
	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
)

// GatewayActorCmdStreamBind 绑定 gateway 建立的 stream。
const GatewayActorCmdStreamBind xactor.CMD = 100

func (p *Gateway) BindStream(stream pb.OnlineService_OnlineStreamTunnelServer) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), GatewayActorCmdStreamBind, stream))
}

// GatewayActorCmdStreamSend 向 gateway stream 发送一帧下行数据。
const GatewayActorCmdStreamSend xactor.CMD = 101

func (p *Gateway) Send(frame *pb.OnlineTunnelFrame) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), GatewayActorCmdStreamSend, frame))
}

// GatewayActorCmdStreamReset stream 断开时清空匹配的 stream。
const GatewayActorCmdStreamReset xactor.CMD = 102

func (p *Gateway) ResetStream(stream pb.OnlineService_OnlineStreamTunnelServer) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), GatewayActorCmdStreamReset, stream))
}

func (p *Gateway) streamBehavior(messages ...any) (xactor.Behavior, any, error) {
	for _, raw := range messages {
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case GatewayActorCmdStreamBind:
			stream, ok := msg.Args[0].(pb.OnlineService_OnlineStreamTunnelServer)
			if !ok {
				continue
			}
			p.stream = stream
			xlog.GLog.Infof("gateway[%s] stream bind", p.Key)
		case GatewayActorCmdStreamSend:
			frame, ok := msg.Args[0].(*pb.OnlineTunnelFrame)
			if !ok {
				continue
			}
			if p.stream == nil {
				xlog.GLog.Errorf("gateway[%s] stream not ready, drop frame uid=%d", p.Key, frame.GetUid())
				continue
			}
			if err := p.stream.Send(&pb.OnlineStreamTunnelRes{Frames: []*pb.OnlineTunnelFrame{frame}}); err != nil {
				xlog.GLog.Errorf("gateway[%s] stream send frame uid=%d err=%v", p.Key, frame.GetUid(), err)
				p.stream = nil
			}
		case GatewayActorCmdStreamReset:
			stream, ok := msg.Args[0].(pb.OnlineService_OnlineStreamTunnelServer)
			if !ok {
				continue
			}
			if p.stream == stream {
				p.stream = nil
				xlog.GLog.Infof("gateway[%s] stream reset", p.Key)
			}
		}
	}
	return p.streamBehavior, nil, nil
}
