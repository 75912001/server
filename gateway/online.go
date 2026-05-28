package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// Online 是一个 online 服务实例。
type Online struct {
	*pb.XOnlineService
	ID string // etcd key

	actor *xactor.Actor[string] // 序列化 stream.Send 的 actor（每个 Online 独立一个）

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
	o.actor = xactor.NewActor[string](id, nil, o.streamBehavior)
	o.actor.Start()
	return o, nil
}

// Send 将消息帧异步投递到 actor，由 actor goroutine 串行调用 stream.Send。
func (p *Online) Send(req *pb.OnlineStreamTunnelReq) error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineActorCmdStreamSend, req))
	return nil
}

// GetID 实现 xgrpcutil.IClientConn，返回服务实例唯一标识。
func (p *Online) GetID() string { return p.ID }

// Stop 停止 actor，关闭底层流和连接。
func (p *Online) Stop() error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	return p.XOnlineService.Stop()
}
