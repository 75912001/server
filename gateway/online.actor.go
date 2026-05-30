package main

import (
	pb "server/proto/pb"

	"context"
	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
)

// OnlineActorCmdStreamSend 向 online stream 发送一批上行帧。
const OnlineActorCmdStreamSend xactor.CMD = 0

// Send 将消息帧异步投递到 actor，由 actor goroutine 串行调用 stream.Send。
func (p *Online) Send(req *pb.OnlineStreamTunnelReq) error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineActorCmdStreamSend, req))
	return nil
}

// OnlineActorCmdStreamReset 流断开时清空匹配的 stream 指针（由 Post 回调投递）。
const OnlineActorCmdStreamReset xactor.CMD = 1

func (p *Online) ResetStream(incoming pb.OnlineService_OnlineStreamTunnelClient) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineActorCmdStreamReset, incoming))
}

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
				xlog.GLog.Errorf("online[%s] stream not ready, drop msg req:%v", p.Key, req)
				continue
			}
			if err := stream.Send(req); err != nil {
				// todo menglc 这里断开后,没有重连机制, 需要后续添加重连机制
				p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient = nil
				xlog.GLog.Errorf("online[%s] stream send req: %v error: %v", p.Key, req, err)
			}
		case OnlineActorCmdStreamReset:
			incoming, ok := msg.Args[0].(pb.OnlineService_OnlineStreamTunnelClient)
			if !ok {
				continue
			}
			if p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient == incoming {
				xlog.GLog.Fatalf("online[%s] stream reset err:%v", p.Key, msg.Args[0])
				// todo menglc 这里断开后,没有重连机制, 需要后续添加重连机制
				p.XStreamOnlineServiceOnlineStreamTunnelClient.OnlineService_OnlineStreamTunnelClient = nil
			}
		}
	}
	return p.streamBehavior, nil, nil
}
