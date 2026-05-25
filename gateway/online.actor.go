package main

import (
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
)

const (
	// OnlineActorCmdStreamSend 参数：*pb.OnlineStreamTunnelReq；向 online stream 发送一批上行帧。
	OnlineActorCmdStreamSend xactor.CMD = 0
	// OnlineActorCmdStreamReset 参数：pb.OnlineService_OnlineStreamTunnelClient；流断开时清空匹配的 stream 指针（由 Post 回调投递）。
	OnlineActorCmdStreamReset xactor.CMD = 1
)

// streamBehavior 是 actor 的唯一消息处理入口，运行在独立 goroutine 中。
// stream.Send 和 OnlineActorCmdStreamReset 在 actor 中串行处理，避免并发发送同一个 gRPC stream。
func (p *Online) streamBehavior(messages ...any) (xactor.Behavior, any, error) {
	for _, raw := range messages {
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case OnlineActorCmdStreamSend:
			req, ok := msg.Args[0].(*pb.OnlineStreamTunnelReq)
			if !ok {
				continue
			}
			stream := p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient
			if stream == nil {
				xlog.PrintfErr("online[%s] stream not ready, drop msg", p.ID)
				continue
			}
			if err := stream.Send(req); err != nil {
				p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient = nil
				xlog.PrintfErr("online[%s] stream send error: %v", p.ID, err)
			}
		case OnlineActorCmdStreamReset:
			incoming, ok := msg.Args[0].(pb.OnlineService_OnlineStreamTunnelClient)
			if !ok {
				continue
			}
			if p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient == incoming {
				p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient = nil
			}
		}
	}
	return p.streamBehavior, nil, nil
}
