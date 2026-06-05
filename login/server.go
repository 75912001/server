package main

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
	xserver "github.com/75912001/xlib/server"
	"github.com/pkg/errors"
)

// LoginServer 组合 xlib Server 和 login 自己的 HTTP 服务生命周期。
type LoginServer struct {
	*xserver.Server

	HTTPServer *http.Server // login HTTP 服务
	httpErrCh  chan error   // HTTP goroutine 退出错误
	httpErr    error        // HTTP 服务最终退出错误
	httpMu     sync.Mutex   // 保护 httpErr
}

// NewLoginServer 创建 login 服务实例并绑定 xlib derived 生命周期。
func NewLoginServer(args []string) *LoginServer {
	srv := xserver.NewServer(args)
	if srv == nil {
		return nil
	}
	initCustomConfig()
	loginServer := &LoginServer{
		Server:    srv,
		httpErrCh: make(chan error, 1),
	}
	srv.Derived = loginServer
	return loginServer
}

// PreStart 初始化 xlib 服务、etcd 监听回调和 HTTP server。
func (p *LoginServer) PreStart(ctx context.Context, opts ...*xserver.Options) error {
	opts = append(opts, xserver.NewServerOptions().
		WithLogCallbackFunc(xcontrol.NewCallBack(func(args ...any) error { return nil })).
		WithEtcd(xetcd.NewOptions().
			WithAddCallback(xcontrol.NewCallBack(onEtcdAdd)).
			WithUpdateCallback(xcontrol.NewCallBack(onEtcdUpdate)).
			WithDelCallback(xcontrol.NewCallBack(onEtcdDel))),
	)
	if err := p.Server.PreStart(ctx, opts...); err != nil {
		return errors.WithMessagef(err, "pre start server failed %v", xruntime.Location())
	}
	p.HTTPServer = newHTTPServer()
	return nil
}

// Start 启动 xlib 服务后异步启动 login HTTP 服务。
func (p *LoginServer) Start(ctx context.Context) error {
	if err := p.Server.Start(ctx); err != nil {
		return errors.WithMessagef(err, "start server failed %v", xruntime.Location())
	}
	if p.HTTPServer == nil {
		return errors.Errorf("http server is nil %v", xruntime.Location())
	}
	go p.serveHTTP()
	return nil
}

// PostStart 进入 xlib 服务启动后的阻塞流程。
func (p *LoginServer) PostStart() error {
	return p.Server.PostStart()
}

// PreStop 优雅关闭 HTTP 服务。
func (p *LoginServer) PreStop() error {
	return p.shutdownHTTP()
}

// Stop 关闭 cache/gateway 连接并停止 xlib 服务。
func (p *LoginServer) Stop() error {
	GCacheMgr.StopAll()
	GGatewayMgr.StopAll()
	if err := p.Server.Stop(); err != nil {
		return err
	}
	p.httpMu.Lock()
	defer p.httpMu.Unlock()
	return p.httpErr
}

// serveHTTP 在独立 goroutine 中运行 HTTP 服务，异常退出时触发服务关闭。
func (p *LoginServer) serveHTTP() {
	xlog.GLog.Infof("login http listen addr:%s pid:%d", GCfgCustomHTTPAddr, os.Getpid())
	err := p.HTTPServer.ListenAndServe()
	if err == http.ErrServerClosed {
		err = nil
	}
	p.httpErrCh <- err
	if err != nil {
		p.Shutdown()
	}
}

// shutdownHTTP 优雅停止 HTTP 服务，并等待 serveHTTP goroutine 退出。
func (p *LoginServer) shutdownHTTP() error {
	if p.HTTPServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), GCfgCustomShutdownTimeout)
	defer cancel()
	if err := p.HTTPServer.Shutdown(ctx); err != nil {
		return errors.WithMessage(err, "http shutdown failed")
	}

	select {
	case err := <-p.httpErrCh:
		p.httpMu.Lock()
		p.httpErr = err
		p.httpMu.Unlock()
		return err
	case <-time.After(GCfgCustomShutdownTimeout):
		return errors.Errorf("http shutdown wait timeout %v", xruntime.Location())
	}
}
