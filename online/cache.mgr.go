package main

import (
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
)

var GCacheMgr = &CacheMgr{
	m: xmap.NewMapMutexMgr[string, *Cache](),
}

type CacheMgr struct {
	m *xmap.MapMutexMgr[string, *Cache]
}

func (p *CacheMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := *gs.PackageName
	serviceName := *gs.ServiceName
	addr := *gs.Addr

	cache, err := newCache(key, addr)
	if err != nil {
		xlog.GLog.Errorf("CacheMgr.Add dial %s failed: %v", addr, err)
		return err
	}
	cache.GroupID = groupID
	cache.ServerName = serverName
	cache.ServerID = serverID
	cache.PackageName = packageName
	cache.ServiceName = serviceName

	p.m.Add(key, cache)
	xgrpcresolve.AddServer(groupID, serverName, serverID, cache, cache.PackageName, cache.ServiceName)

	xlog.GLog.Infof("CacheMgr.Add id=%s addr=%s total=%d", key, *gs.Addr, p.m.Len())
	return nil
}

func (p *CacheMgr) Remove(key string) {
	cache, ok := p.m.Find(key)
	if !ok {
		return
	}

	if _, err := xgrpcresolve.RemoveServer(cache.GroupID, cache.ServerName, cache.ServerID, cache.PackageName, cache.ServiceName); err != nil {
		xlog.GLog.Warnf("CacheMgr.Remove RemoveServer key=%s: %v", key, err)
		if stopErr := cache.Stop(); stopErr != nil {
			xlog.GLog.Warnf("CacheMgr.Remove fallback Stop key=%s: %v", key, stopErr)
		}
	}
	p.m.Del(key)

	xlog.GLog.Infof("CacheMgr.removeInfo RemoveServer key:%s total:%v", key, p.m.Len())
}
