package main

import (
	"sync"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
)

var GGatewayMgr = newGatewayMgr()

type GatewayMgr struct {
	mu sync.RWMutex
	m  map[string]*Gateway
}

type Gateway struct {
	Key           string
	Addr          string
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

func (p *GatewayMgr) AddOrUpdate(key string, valueJson *xetcd.ValueJson) {
	if valueJson == nil {
		return
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gateway := &Gateway{
		Key:           key,
		Addr:          extractGatewayAddr(valueJson),
		GroupID:       groupID,
		ServerName:    serverName,
		ServerID:      serverID,
		AvailableLoad: valueJson.AvailableLoad,
	}

	p.mu.Lock()
	p.m[key] = gateway
	total := len(p.m)
	p.mu.Unlock()

	xlog.GLog.Infof("GatewayMgr.AddOrUpdate key:%s addr:%s availableLoad:%d total:%d", key, gateway.Addr, gateway.AvailableLoad, total)
}

func (p *GatewayMgr) Remove(key string) {
	p.mu.Lock()
	delete(p.m, key)
	total := len(p.m)
	p.mu.Unlock()

	xlog.GLog.Infof("GatewayMgr.Remove key:%s total:%d", key, total)
}

func (p *GatewayMgr) GetByAvailableLoad() (*Gateway, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var selected *Gateway
	for key, gateway := range p.m {
		if gateway == nil || gateway.Addr == "" || gateway.AvailableLoad == 0 {
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
