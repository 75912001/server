package main

import (
	"context"
	"server/common"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xserver "github.com/75912001/xlib/server"
)

type Gateway struct {
	*xserver.Server
}

// NewGateway 解析配置并创建服务实例
// args: [0:程序名] [1:配置文件绝对路径]
func NewGateway(args []string) *Gateway {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	return &Gateway{Server: srv}
}

// PreStart 配置 TCPHandler / HeaderStrategy / etcd 回调，再调用 xlib server 完成日志/actor/timer 初始化
func (g *Gateway) PreStart(ctx context.Context) error {
	// 初始化 gRPC selector：扫描 protoregistry 中所有服务/方法选项，建立负载均衡策略表
	// 必须在第一次调用 selector.Sel（即 XOnlineServiceClient.OnlineUserOnline）之前完成
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()

	opts := xserver.NewServerOptions().
		WithTCPHandler(GUserHandlerTCP).
		WithHeaderStrategy(&common.DefaultHeaderStrategy{}).
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil })).
		WithEtcd(xetcd.NewOptions().
			WithAddCallback(xcontrol.NewCallBack(onEtcdAdd)).
			WithUpdateCallback(xcontrol.NewCallBack(onEtcdUpdate)).
			WithDelCallback(xcontrol.NewCallBack(onEtcdDel)))

	if err := g.Server.PreStart(ctx, opts); err != nil {
		return err
	}
	if g.Server.GRPCServer != nil {
		pb.RegisterGatewayServiceServer(g.Server.GRPCServer.GrpcServer, &gatewayGRPCServer{})
	}
	return nil
}
