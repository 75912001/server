package main

import (
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
)

var GGatewayMgr = &GatewayMgr{
	m: xmap.NewMapMutexMgr[string, *Gateway](), // key: etcd key
}

type GatewayMgr struct {
	m *xmap.MapMutexMgr[string, *Gateway]
}

func (p *GatewayMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := *gs.PackageName
	serviceName := *gs.ServiceName
	addr := *gs.Addr

	p.Remove(key)

	gateway, err := newGateway(key, *gs.Addr)
	if err != nil {
		xlog.GLog.Errorf("GatewayMgr.Add dial %s failed: %v", addr, err)
		return err
	}
	gateway.GroupID = groupID
	gateway.ServerName = serverName
	gateway.ServerID = serverID
	gateway.PackageName = packageName
	gateway.ServiceName = serviceName

	p.m.Add(key, gateway)

	xgrpcresolve.AddServer(groupID, serverName, serverID, gateway, packageName, serviceName)

	xlog.GLog.Infof("GatewayMgr.Add key:%s addr:%s total:%d", key, addr, p.m.Len())
	return nil
}

func (p *GatewayMgr) Remove(key string) {
	gateway, ok := p.m.Find(key)
	if !ok {
		return
	}
	if _, err := xgrpcresolve.RemoveServer(
		gateway.GroupID, gateway.ServerName, gateway.ServerID,
		gateway.PackageName, gateway.ServiceName,
	); err != nil {
		xlog.GLog.Warnf("GatewayMgr.Remove RemoveServer key=%s: %v", key, err)
		if stopErr := gateway.Stop(); stopErr != nil {
			xlog.GLog.Warnf("GatewayMgr.Remove fallback Stop key=%s: %v", key, stopErr)
		}
	}
	p.m.Del(key)
}

func (p *GatewayMgr) Get(id string) *Gateway {
	gateway, _ := p.m.Find(id)
	return gateway
}

func (p *GatewayMgr) Len() int {
	return p.m.Len()
}
