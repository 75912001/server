package main

import (
	"context"
	"server/common"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xruntime "github.com/75912001/xlib/runtime"
	xserver "github.com/75912001/xlib/server"
	"github.com/pkg/errors"
	"google.golang.org/grpc/reflection"
)

type GatewayServer struct {
	*xserver.Server
}

// NewGatewayServer 解析配置并创建服务实例
// args: [0:程序名] [1:配置文件绝对路径]
func NewGatewayServer(args []string) *GatewayServer {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	initCustomConfig()
	return &GatewayServer{Server: srv}
}

// PreStart 配置 TCPHandler / HeaderStrategy / etcd 回调，再调用 xlib server 完成日志/actor/timer 初始化
func (p *GatewayServer) PreStart(ctx context.Context) error {
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

	if err := p.Server.PreStart(ctx, opts); err != nil {
		return errors.WithMessagef(err, "pre start server failed, %v %v", opts, xruntime.Location())
	}

	if p.Server.GRPCServer != nil {
		pb.RegisterGatewayServiceServer(p.Server.GRPCServer.GrpcServer, &gatewayGRPCServer{})

		if xruntime.IsDebug() {
			// grpcurl -plaintext ip:port list gateway.GatewayService
			// grpcurl -plaintext ip:port describe gateway.GatewayService
			reflection.Register(p.Server.GRPCServer.GrpcServer)
		}
	}
	return nil
}
