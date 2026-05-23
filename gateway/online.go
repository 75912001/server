package main

import (
	"context"
	"sync"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

const (
	CmdStreamSend  xactor.CMD = 0 // 向 online stream 发送一帧
	CmdStreamReset xactor.CMD = 1 // 流断开时清空 stream 指针（由 Post 回调投递）
)

// Online 是一个 online 服务实例。
type Online struct {
	*pb.XOnlineService
	ID string // ${GroupID}.${serverName}.${serverID}

	actor  *xactor.Actor[string]                     // 序列化 stream.Send 的 actor（每个 Online 独立一个）
	stream pb.OnlineService_OnlineStreamTunnelClient // 仅 actor goroutine 访问，无需加锁

	closeOnce sync.Once // 保证 actor Stop 只发送一次

	GroupID     uint32
	ServerName  string
	ServerID    uint32
	PackageName string
	ServiceName string
}

// newOnline 建立 gRPC 连接，启动 recvLoop 和 stream actor。
func newOnline(id, addr string) (*Online, error) {
	xService, err := pb.NewXOnlineService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	o := &Online{ID: id, XOnlineService: xService}
	_ = xService.Start()
	o.stream = xService.GetStream()
	o.actor = xactor.NewActor[string](id, nil, o.streamBehavior)
	o.actor.Start()
	return o, nil
}

// streamBehavior 是 actor 的唯一消息处理入口，运行在独立 goroutine 中。
// stream 字段仅在此函数内读写，无需任何锁。
func (p *Online) streamBehavior(messages ...any) (xactor.Behavior, any, error) {
	for _, raw := range messages {
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case CmdStreamSend:
			req, ok := msg.Args[0].(*pb.OnlineStreamTunnelReq)
			if !ok {
				continue
			}
			if p.stream == nil {
				xlog.PrintfErr("online[%s] stream not ready, drop msg", p.ID)
				continue
			}
			if err := p.stream.Send(req); err != nil {
				p.stream = nil
				xlog.PrintfErr("online[%s] stream send error: %v", p.ID, err)
			}
		case CmdStreamReset:
			incoming, ok := msg.Args[0].(pb.OnlineService_OnlineStreamTunnelClient)
			if !ok {
				continue
			}
			if p.stream == incoming {
				p.stream = nil
			}
		}
	}
	return p.streamBehavior, nil, nil
}

// Send 将消息帧异步投递到 actor，由 actor goroutine 串行调用 stream.Send。
func (p *Online) Send(req *pb.OnlineStreamTunnelReq) error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), CmdStreamSend, req))
	return nil
}

// GetID 实现 xgrpcutil.IClientConn，返回服务实例唯一标识。
func (p *Online) GetID() string { return p.ID }

// Stop 标记不可用，停止 actor，关闭底层流和连接。
func (p *Online) Stop() error {
	p.Disabled()
	p.closeOnce.Do(func() {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	})
	return p.XOnlineService.Stop()
}
