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
	Key string // etcd key

	actor *xactor.Actor[string] // 序列化 stream.Send 的 actor（每个 Online 独立一个）

	GroupID       uint32
	ServerName    string
	ServerID      uint32
	PackageName   string
	ServiceName   string
	AvailableLoad uint32
}

// newOnline 建立 gRPC 连接，启动 recvLoop 和 stream actor。
func newOnline(key, addr string) (*Online, error) {
	xService, err := pb.NewXOnlineService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	o := &Online{Key: key, XOnlineService: xService}
	_ = xService.Start()
	o.actor = xactor.NewActor[string](key, nil, o.streamBehavior)
	o.actor.Start()
	return o, nil
}

// GetID 实现 xgrpcutil.IClientConn，返回服务实例唯一标识。
func (p *Online) GetID() string { return p.Key }

// Stop 停止 actor，关闭底层流和连接。
func (p *Online) Stop() error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	err := p.XOnlineService.Stop()
	if err != nil {
		return errors.WithMessagef(err, "stop XOnlineService error. %v", xruntime.Location())
	}
	return nil
}
