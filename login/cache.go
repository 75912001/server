package main

import (
	"sync"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var GCacheMgr = newCacheMgr()

type CacheMgr struct {
	mu sync.Mutex
	m  map[string]*Cache
}

type Cache struct {
	*pb.XCacheService
	Key string

	GroupID     uint32
	ServerName  string
	ServerID    uint32
	PackageName string
	ServiceName string
}

func newCacheMgr() *CacheMgr {
	return &CacheMgr{
		m: make(map[string]*Cache),
	}
}

func newCache(key, addr string) (*Cache, error) {
	xService, err := pb.NewXCacheService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	return &Cache{Key: key, XCacheService: xService}, nil
}

func (p *CacheMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	cache, err := newCache(key, *gs.Addr)
	if err != nil {
		return err
	}
	cache.GroupID = groupID
	cache.ServerName = serverName
	cache.ServerID = serverID
	cache.PackageName = *gs.PackageName
	cache.ServiceName = *gs.ServiceName

	p.mu.Lock()
	p.m[key] = cache
	total := len(p.m)
	p.mu.Unlock()

	xgrpcresolve.AddServer(groupID, serverName, serverID, cache, cache.PackageName, cache.ServiceName)
	xlog.GLog.Infof("CacheMgr.Add key:%s addr:%s total:%d", key, *gs.Addr, total)
	return nil
}

func (p *CacheMgr) Remove(key string) {
	p.mu.Lock()
	cache, ok := p.m[key]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.m, key)
	total := len(p.m)
	p.mu.Unlock()

	if _, err := xgrpcresolve.RemoveServer(cache.GroupID, cache.ServerName, cache.ServerID, cache.PackageName, cache.ServiceName); err != nil {
		xlog.GLog.Warnf("CacheMgr.Remove RemoveServer key:%s err:%v", key, err)
	}
	if err := cache.Stop(); err != nil {
		xlog.GLog.Warnf("CacheMgr.Remove Stop key:%s err:%v", key, err)
	}
	xlog.GLog.Infof("CacheMgr.Remove key:%s total:%d", key, total)
}

func (p *CacheMgr) StopAll() {
	p.mu.Lock()
	keys := make([]string, 0, len(p.m))
	for key := range p.m {
		keys = append(keys, key)
	}
	p.mu.Unlock()

	for _, key := range keys {
		p.Remove(key)
	}
}

func (p *Cache) GetID() string { return p.Key }

func (p *Cache) GetClientConn() *grpc.ClientConn {
	return p.XCacheService.GetClientConn()
}
