package main

import (
	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// GCacheMgr 管理 login 发现到的 cache 节点，并同步注册到 xlib gRPC resolver。
var GCacheMgr = newCacheMgr()

// CacheMgr 保存 cache 节点本地索引；写入只来自 login 的 etcd 回调顺序事件。
type CacheMgr struct {
	m map[string]*Cache // key: etcd server key
}

// Cache 是 login 侧缓存的 cache 节点连接信息。
type Cache struct {
	*pb.XCacheService
	Key string // cache 在 etcd 中的 server key

	GroupID     uint32 // etcd 分组 ID
	ServerName  string // 服务名
	ServerID    uint32 // 服务实例 ID
	PackageName string // gRPC package 名
	ServiceName string // gRPC service 名
}

// newCacheMgr 创建 cache 节点管理器。
func newCacheMgr() *CacheMgr {
	return &CacheMgr{
		m: make(map[string]*Cache),
	}
}

// newCache 创建到 cache gRPC 服务的 X 客户端连接。
func newCache(key, addr string) (*Cache, error) {
	xService, err := pb.NewXCacheService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	return &Cache{Key: key, XCacheService: xService}, nil
}

// Add 将 cache 节点加入本地索引，并注册到 xlib resolver 供 GXCacheServiceService 使用。
func (p *CacheMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
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

	p.m[key] = cache
	total := len(p.m)

	xgrpcresolve.AddServer(groupID, serverName, serverID, cache, cache.PackageName, cache.ServiceName)
	xlog.GLog.Infof("CacheMgr.Add key:%s addr:%s total:%d", key, *gs.Addr, total)
	return nil
}

// Remove 将 cache 节点从本地索引和 xlib resolver 中移除，并关闭连接。
func (p *CacheMgr) Remove(key string) {
	cache, ok := p.m[key]
	if !ok {
		return
	}
	delete(p.m, key)
	total := len(p.m)

	if _, err := xgrpcresolve.RemoveServer(cache.GroupID, cache.ServerName, cache.ServerID, cache.PackageName, cache.ServiceName); err != nil {
		xlog.GLog.Warnf("CacheMgr.Remove RemoveServer key:%s err:%v", key, err)
	}
	if err := cache.Stop(); err != nil {
		xlog.GLog.Warnf("CacheMgr.Remove Stop key:%s err:%v", key, err)
	}
	xlog.GLog.Infof("CacheMgr.Remove key:%s total:%d", key, total)
}

// StopAll 停止当前 login 已发现的所有 cache 连接。
func (p *CacheMgr) StopAll() {
	keys := make([]string, 0, len(p.m))
	for key := range p.m {
		keys = append(keys, key)
	}

	for _, key := range keys {
		p.Remove(key)
	}
}

// GetID 返回 resolver 使用的 cache 节点 ID。
func (p *Cache) GetID() string { return p.Key }

// GetClientConn 返回 cache 节点的底层 gRPC 连接。
func (p *Cache) GetClientConn() *grpc.ClientConn {
	return p.XCacheService.GetClientConn()
}
