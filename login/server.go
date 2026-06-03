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

type LoginServer struct {
	*xserver.Server

	HTTPServer *http.Server
	httpErrCh  chan error
	httpErr    error
	httpMu     sync.Mutex
}

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

func (p *LoginServer) PostStart() error {
	return p.Server.PostStart()
}

func (p *LoginServer) PreStop() error {
	return p.shutdownHTTP()
}

func (p *LoginServer) Stop() error {
	GCacheMgr.StopAll()
	if err := p.Server.Stop(); err != nil {
		return err
	}
	p.httpMu.Lock()
	defer p.httpMu.Unlock()
	return p.httpErr
}

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
