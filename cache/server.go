package main

import (
	"context"

	pb "server/proto/pb"

	xconfig "github.com/75912001/xlib/config"
	xcontrol "github.com/75912001/xlib/control"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xruntime "github.com/75912001/xlib/runtime"
	xserver "github.com/75912001/xlib/server"
	"github.com/pkg/errors"
	"google.golang.org/grpc/reflection"
)

type CacheServer struct {
	*xserver.Server
}

func NewCacheServer(args []string) *CacheServer {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	initCustomConfig()
	return &CacheServer{Server: srv}
}

func (p *CacheServer) PreStart(ctx context.Context) error {
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()

	{ // 初始化 Redis 客户端
		var err error
		GRedis, err = newRedis(xconfig.GConfigMgr.Redis)
		if err != nil {
			return errors.WithMessagef(err, "newRedis err %v", xruntime.Location())
		}
		if err = GRedis.Ping(ctx); err != nil {
			return errors.WithMessagef(err, "redis ping err %v", xruntime.Location())
		}
	}

	opts := xserver.NewServerOptions().
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil }))
	if err := p.Server.PreStart(ctx, opts); err != nil {
		return errors.WithMessagef(err, "pre start server failed, %v %v", opts, xruntime.Location())
	}

	if p.Server.GRPCServer != nil {
		pb.RegisterCacheServiceServer(p.Server.GRPCServer.GrpcServer, &cacheGRPCServer{})

		if xruntime.IsDebug() {
			// grpcurl -plaintext localhost:20301 list cache.CacheService
			// grpcurl -plaintext localhost:20301 describe cache.CacheService
			reflection.Register(p.Server.GRPCServer.GrpcServer)
		}
	}
	return nil
}
