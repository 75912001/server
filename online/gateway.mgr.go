package main

import (
	pb "server/proto/pb"
	"sync"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
)

var GGatewayMgr = &GatewayMgr{
	m: xmap.NewMapMutexMgr[string, *Gateway](), // key: etcd key
}

type GatewayMgr struct {
	m  *xmap.MapMutexMgr[string, *Gateway]
	mu sync.Mutex // etcd 消息和 stream 注册消息可能并发到达，保护查找/创建/删除 Gateway 的复合操作。
}

func (p *GatewayMgr) AddByEtcd(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := *gs.PackageName
	serviceName := *gs.ServiceName
	addr := *gs.Addr

	p.mu.Lock()
	gateway := p.Get(key)
	if gateway == nil {
		gateway = newGateway(key)
		p.m.Add(key, gateway)
	}
	if gateway.PackageName != "" && gateway.ServiceName != "" {
		if _, err := xgrpcresolve.RemoveServer(
			gateway.GroupID, gateway.ServerName, gateway.ServerID,
			gateway.PackageName, gateway.ServiceName,
		); err != nil {
			xlog.GLog.Warnf("GatewayMgr.AddByEtcd RemoveServer old key=%s: %v", key, err)
		}
	}
	if err := gateway.UpdateService(addr, groupID, serverName, serverID, packageName, serviceName); err != nil {
		p.mu.Unlock()
		xlog.GLog.Errorf("GatewayMgr.AddByEtcd dial %s failed: %v", addr, err)
		return err
	}
	p.mu.Unlock()

	xgrpcresolve.AddServer(groupID, serverName, serverID, gateway, packageName, serviceName)

	xlog.GLog.Infof("GatewayMgr.AddByEtcd key:%s addr:%s total:%d", key, addr, p.m.Len())
	return nil
}

func (p *GatewayMgr) Remove(key string) {
	p.mu.Lock()
	defer func() {
		p.mu.Unlock()
	}()
	gateway, ok := p.m.Find(key)
	if !ok {
		return
	}
	if gateway.PackageName != "" && gateway.ServiceName != "" {
		if _, err := xgrpcresolve.RemoveServer(
			gateway.GroupID, gateway.ServerName, gateway.ServerID,
			gateway.PackageName, gateway.ServiceName,
		); err != nil {
			xlog.GLog.Warnf("GatewayMgr.Remove RemoveServer key=%s: %v", key, err)
		}
	}
	if err := gateway.Stop(); err != nil {
		xlog.GLog.Warnf("GatewayMgr.Remove Stop key=%s: %v", key, err)
	}
	p.m.Del(key)
}

func (p *GatewayMgr) Get(id string) *Gateway {
	gateway, _ := p.m.Find(id)
	return gateway
}

func (p *GatewayMgr) BindStream(id string, stream pb.OnlineService_OnlineStreamTunnelServer) *Gateway {
	p.mu.Lock()
	gateway := p.Get(id)
	if gateway == nil {
		gateway = newGateway(id)
		p.m.Add(id, gateway)
	}
	p.mu.Unlock()
	gateway.BindStream(stream)
	return gateway
}

func (p *GatewayMgr) ResetStream(id string, stream pb.OnlineService_OnlineStreamTunnelServer) {
	p.mu.Lock()
	if gateway := p.Get(id); gateway != nil {
		gateway.ResetStream(stream)
	}
	p.mu.Unlock()
}

func (p *GatewayMgr) Len() int {
	return p.m.Len()
}
