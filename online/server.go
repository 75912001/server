package main

import (
	"context"
	"sync"
	"time"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
	xserver "github.com/75912001/xlib/server"
	"github.com/pkg/errors"
	"google.golang.org/grpc/reflection"
)

// onlineShutdownFlushBudget 是停服落盘的总时间预算, 必须小于容器 terminationGracePeriodSeconds.
const onlineShutdownFlushBudget = 5 * time.Second

// onlineShutdownFlushConcurrency 是停服落盘的并发度, 避免逐账号串行写把预算耗尽.
const onlineShutdownFlushConcurrency = 16

type OnlineServer struct {
	*xserver.Server
}

// onlineServerDerived 是 xserver 的"派生服务"适配器。
// OnlineServer.PreStart 的签名与 xserver.IServer 不同(不带 options), 因此不能直接把
// OnlineServer 赋给 Derived; 这里用内嵌 Server + 覆写 PreStop 的方式接入停服钩子,
// 其余方法仍委托给内层 Server。
type onlineServerDerived struct {
	*xserver.Server
	preStop func() error
}

func (p *onlineServerDerived) PreStop() error {
	if p.preStop == nil {
		return nil
	}
	return p.preStop()
}

// NewOnlineServer 解析配置并创建服务实例。
// args: [0:程序名称] [1:配置文件绝对路径]
func NewOnlineServer(args []string) *OnlineServer {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	initCustomConfig()
	server := &OnlineServer{Server: srv}
	// xserver 默认让 Derived 指向内部 Server, 这里替换为带停服落盘钩子的适配器。
	srv.Derived = &onlineServerDerived{Server: srv, preStop: server.PreStop}
	return server
}

// PreStop 在服务关闭前把全部在线账号的档案同步落盘。
// 延迟落盘把窗口内的修改留在内存, 不做这一步会让每次发版都造成计划内回档。
func (p *OnlineServer) PreStop() error {
	accounts := make([]*Account, 0, GAccountMgr.accounts.Len())
	GAccountMgr.accounts.Foreach(func(_ uint64, account *Account) bool {
		if account != nil {
			accounts = append(accounts, account)
		}
		return true
	})
	if len(accounts) == 0 {
		return nil
	}

	start := time.Now()
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, onlineShutdownFlushConcurrency)
	for _, account := range accounts {
		waitGroup.Add(1)
		semaphore <- struct{}{}
		go func(target *Account) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			// Account.Stop 会同步投递 CmdStop, 在 actor 内完成落盘后才返回,
			// 避免停服遍历与 actor 并发写同一份账号档案。
			target.Stop()
		}(account)
	}

	flushed := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(flushed)
	}()

	select {
	case <-flushed:
		xlog.GLog.Infof("online shutdown flush done accounts=%d cost=%s", len(accounts), time.Since(start))
	case <-time.After(onlineShutdownFlushBudget):
		xlog.GLog.Warnf("online shutdown flush exceeded budget accounts=%d cost=%s, 剩余账号未落盘",
			len(accounts), time.Since(start))
	}
	return nil
}

// PreStart 配置 gRPC selector / etcd 回调，再调用 xlib server 完成日志/actor/timer 初始化，并注册 OnlineService。
func (p *OnlineServer) PreStart(ctx context.Context) error {
	var err error
	err = gameconfig.Load(GCfgCustomGameConfigDir)
	if err != nil {
		return errors.WithMessagef(err, "load game config failed, dir:%s %v", GCfgCustomGameConfigDir, xruntime.Location())
	}
	if err := validateEnemyCombatSkillConfig(); err != nil {
		return errors.WithMessagef(err, "validate enemy combat skill config failed, dir:%s %v", GCfgCustomGameConfigDir, xruntime.Location())
	}

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
