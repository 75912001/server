package main

import (
	"sync"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

var GGatewayMgr = newGatewayMgr()

type GatewayMgr struct {
	mu sync.RWMutex
	m  map[string]*Gateway
}

type Gateway struct {
	*pb.XGatewayService

	Key           string
	Addr          string
	GrpcAddr      string
	GroupID       uint32
	ServerName    string
	ServerID      uint32
	AvailableLoad uint32
}

func newGatewayMgr() *GatewayMgr {
	return &GatewayMgr{
		m: make(map[string]*Gateway),
	}
}

func (p *GatewayMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	addr := extractGatewayAddr(valueJson)
	grpcAddr := extractGatewayGRPCAddr(valueJson)
	if grpcAddr == "" {
		return errors.Errorf("gateway grpc addr is empty key:%s %v", key, xruntime.Location())
	}

	xGatewayService, err := pb.NewXGatewayService(grpcAddr)
	if err != nil {
		return errors.WithMessagef(err, "new gateway service key:%s addr:%s %v", key, grpcAddr, xruntime.Location())
	}

	gateway := &Gateway{
		XGatewayService: xGatewayService,
		Key:             key,
		Addr:            addr,
		GrpcAddr:        grpcAddr,
		GroupID:         groupID,
		ServerName:      serverName,
		ServerID:        serverID,
		AvailableLoad:   valueJson.AvailableLoad,
	}

	p.mu.Lock()
	old := p.m[key]
	p.m[key] = gateway
	total := len(p.m)
	p.mu.Unlock()

	p.stopGateway(old)
	xlog.GLog.Infof("GatewayMgr.Add key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, gateway.Addr, gateway.GrpcAddr, gateway.AvailableLoad, total)
	return nil
}

func (p *GatewayMgr) Update(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	addr := extractGatewayAddr(valueJson)
	grpcAddr := extractGatewayGRPCAddr(valueJson)
	if grpcAddr == "" {
		p.Remove(key)
		return nil
	}

	p.mu.Lock()
	old := p.m[key]
	if old != nil && old.GrpcAddr == grpcAddr && old.XGatewayService != nil {
		old.Addr = addr
		old.GroupID = groupID
		old.ServerName = serverName
		old.ServerID = serverID
		old.AvailableLoad = valueJson.AvailableLoad
		total := len(p.m)
		p.mu.Unlock()

		xlog.GLog.Infof("GatewayMgr.Update reuse key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, old.Addr, old.GrpcAddr, old.AvailableLoad, total)
		return nil
	}
	p.mu.Unlock()

	return p.Add(key, valueJson)
}

func (p *GatewayMgr) Remove(key string) {
	p.mu.Lock()
	gateway, ok := p.m[key]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.m, key)
	total := len(p.m)
	p.mu.Unlock()

	p.stopGateway(gateway)
	xlog.GLog.Infof("GatewayMgr.Remove key:%s total:%d", key, total)
}

func (p *GatewayMgr) StopAll() {
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

func (p *GatewayMgr) GetByAvailableLoad() (*Gateway, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var selected *Gateway
	for key, gateway := range p.m {
		if gateway == nil || gateway.Addr == "" || gateway.GrpcAddr == "" ||
			gateway.XGatewayService == nil || !gateway.Available() || gateway.AvailableLoad == 0 {
			continue
		}
		if selected == nil ||
			gateway.AvailableLoad > selected.AvailableLoad ||
			(gateway.AvailableLoad == selected.AvailableLoad && key < selected.Key) {
			cp := *gateway
			selected = &cp
		}
	}
	return selected, selected != nil
}

func (p *GatewayMgr) stopGateway(gateway *Gateway) {
	if gateway == nil || gateway.XGatewayService == nil {
		return
	}
	gateway.Disabled()
	if err := gateway.Stop(); err != nil {
		xlog.GLog.Warnf("GatewayMgr.Stop key:%s err:%v", gateway.Key, err)
	}
}

func extractGatewayGRPCAddr(valueJson *xetcd.ValueJson) string {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return ""
	}
	return *valueJson.GrpcService.Addr
}

func extractGatewayAddr(valueJson *xetcd.ValueJson) string {
	for _, serverNet := range valueJson.ServerNet {
		if serverNet == nil || serverNet.Addr == nil || *serverNet.Addr == "" {
			continue
		}
		netType := xnetcommon.ServerNetTypeNameTCP
		if serverNet.Type != nil {
			netType = *serverNet.Type
		}
		if netType == xnetcommon.ServerNetTypeNameTCP {
			return *serverNet.Addr
		}
	}
	return ""
}
