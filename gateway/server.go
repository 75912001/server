package main

import (
	"context"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
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
	opts := xserver.NewServerOptions().
		WithTCPHandler(GUserHandlerTCP).
		WithHeaderStrategy(&DefaultHeaderStrategy{}).
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil })).
		WithEtcd(xetcd.NewOptions().
			WithAddCallback(xcontrol.NewCallBack(onEtcdAdd)).
			WithUpdateCallback(xcontrol.NewCallBack(onEtcdUpdate)).
			WithDelCallback(xcontrol.NewCallBack(onEtcdDel)))

	return g.Server.PreStart(ctx, opts)
}
