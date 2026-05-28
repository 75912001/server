package main

import (
	"context"

	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xruntime "github.com/75912001/xlib/runtime"
	xserver "github.com/75912001/xlib/server"
	"google.golang.org/grpc/reflection"
)

type OnlineServer struct {
	*xserver.Server
}

// NewOnlineServer 解析配置并创建服务实例。
// args: [0:程序名称] [1:配置文件绝对路径]
func NewOnlineServer(args []string) *OnlineServer {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	return &OnlineServer{Server: srv}
}

// PreStart 配置 gRPC selector / etcd 回调，再调用 xlib server 完成日志/actor/timer 初始化，并注册 OnlineService。
func (p *OnlineServer) PreStart(ctx context.Context) error {
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()

	opts := xserver.NewServerOptions().
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil })).
		WithEtcd(xetcd.NewOptions().
			WithAddCallback(xcontrol.NewCallBack(onEtcdAdd)).
			WithUpdateCallback(xcontrol.NewCallBack(onEtcdUpdate)).
			WithDelCallback(xcontrol.NewCallBack(onEtcdDel)))

	if err := p.Server.PreStart(ctx, opts); err != nil {
		return err
	}

	if p.Server.GRPCServer != nil {
		pb.RegisterOnlineServiceServer(p.Server.GRPCServer.GrpcServer, &onlineGRPCServer{})

		if xruntime.IsDebug() {
			// grpcurl -plaintext ip:port list online.OnlineService
			// grpcurl -plaintext ip:port describe online.OnlineService
			reflection.Register(p.Server.GRPCServer.GrpcServer)
		}
	}
	return nil
}
