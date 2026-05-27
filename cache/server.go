package main

import (
	"context"

	pb "server/proto/pb"

	xconfig "github.com/75912001/xlib/config"
	xcontrol "github.com/75912001/xlib/control"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xserver "github.com/75912001/xlib/server"
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

func (s *CacheServer) PreStart(ctx context.Context) error {
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()

	opts := xserver.NewServerOptions().
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil }))
	if err := s.Server.PreStart(ctx, opts); err != nil {
		return err
	}

	// 初始化 Redis 客户端
	var err error
	GRedis, err = newRedis(xconfig.GConfigMgr.Redis)
	if err != nil {
		return err
	}
	if err := GRedis.Ping(ctx); err != nil {
		return err
	}

	if s.Server.GRPCServer != nil {
		pb.RegisterCacheServiceServer(s.Server.GRPCServer.GrpcServer, &cacheGRPCServer{})

		if *xconfig.GConfigMgr.Base.RunMode == 1 { // debug 开发模式，注册 gRPC 反射服务，方便使用 grpcurl 等工具调试
			// grpcurl -plaintext localhost:20301 list cache.CacheService
			// grpcurl -plaintext localhost:20301 describe cache.CacheService
			reflection.Register(s.Server.GRPCServer.GrpcServer)
		}
	}
	return nil
}
