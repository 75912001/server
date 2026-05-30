package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xerror "github.com/75912001/xlib/error"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Gateway 是 gateway 的唯一状态容器。
//
// etcd 消息负责补齐 gRPC service 信息，stream 消息负责绑定下行流；
// 两者到达顺序不固定，因此统一落到同一个 Gateway 对象中分步补齐状态。
// stream 读写只通过 gateway.actor 串行处理，避免多个 user.actor 并发 Send 同一条 stream。
type Gateway struct {
	*pb.XGatewayService
	Key string // etcd key

	actor  *xactor.Actor[string]
	stream pb.OnlineService_OnlineStreamTunnelServer

	GroupID     uint32
	ServerName  string
	ServerID    uint32
	PackageName string
	ServiceName string
}

func newGateway(key string) *Gateway {
	p := &Gateway{Key: key}
	p.actor = xactor.NewActor[string](key, nil, p.streamBehavior)
	p.actor.Start()
	return p
}

func (p *Gateway) UpdateService(addr string, groupID uint32, serverName string, serverID uint32, packageName string, serviceName string) error {
	xService, err := pb.NewXGatewayService(addr)
	if err != nil {
		return err
	}
	if p.XGatewayService != nil {
		_ = p.XGatewayService.Stop()
	}
	p.XGatewayService = xService
	p.GroupID = groupID
	p.ServerName = serverName
	p.ServerID = serverID
	p.PackageName = packageName
	p.ServiceName = serviceName
	return nil
}

func (p *Gateway) Client() (pb.GatewayServiceClient, error) {
	if p.XGatewayService == nil {
		return nil, errors.WithMessagef(xerror.NotExist, "Gateway %v %v", p.Key, xruntime.Location())
	}
	return pb.NewGatewayServiceClient(p.GetClientConn()), nil
}

func (p *Gateway) GetClientConn() *grpc.ClientConn {
	if p.XGatewayService == nil {
		return nil
	}
	return p.XGatewayService.GetClientConn()
}

func (p *Gateway) GetID() string { return p.Key }

func (p *Gateway) Stop() error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	if p.XGatewayService == nil {
		return nil
	}
	err := p.XGatewayService.Stop()
	if err != nil {
		return errors.WithMessagef(err, "stop XGatewayService error. %v", xruntime.Location())
	}
	return nil
}
