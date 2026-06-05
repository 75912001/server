package main

import (
	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

var GGatewayMgr = newGatewayMgr()

type GatewayMgr struct {
	m *xmap.MapMutexMgr[string, *Gateway]
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
		m: xmap.NewMapMutexMgr[string, *Gateway](),
	}
}

func (p *GatewayMgr) Add(key string, valueJson *xetcd.ValueJson) error {
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

	p.m.Add(key, gateway)
	total := p.m.Len()

	xlog.GLog.Infof("GatewayMgr.Add key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, gateway.Addr, gateway.GrpcAddr, gateway.AvailableLoad, total)
	return nil
}

func (p *GatewayMgr) Update(key string, valueJson *xetcd.ValueJson) error {
	_, groupID, serverName, serverID := xetcd.Parse(key)
	addr := extractGatewayAddr(valueJson)
	grpcAddr := extractGatewayGRPCAddr(valueJson)
	if grpcAddr == "" {
		return errors.Errorf("gateway grpc addr is empty key:%s %v", key, xruntime.Location())
	}

	old, _ := p.m.Find(key)
	if old != nil && old.GrpcAddr == grpcAddr && old.XGatewayService != nil {
		old.Addr = addr
		old.GroupID = groupID
		old.ServerName = serverName
		old.ServerID = serverID
		old.AvailableLoad = valueJson.AvailableLoad
		total := p.m.Len()

		xlog.GLog.Infof("GatewayMgr.Update reuse key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, old.Addr, old.GrpcAddr, old.AvailableLoad, total)
		return nil
	}

	return p.Add(key, valueJson)
}

func (p *GatewayMgr) Remove(key string) {
	gateway, ok := p.m.Find(key)
	if !ok {
		return
	}
	p.m.Del(key)
	total := p.m.Len()

	p.stopGateway(gateway)
	xlog.GLog.Infof("GatewayMgr.Remove key:%s total:%d", key, total)
}

func (p *GatewayMgr) StopAll() {
	keys := make([]string, 0, p.m.Len())
	p.m.Foreach(func(key string, _ *Gateway) bool {
		keys = append(keys, key)
		return true
	})

	for _, key := range keys {
		p.Remove(key)
	}
}

func (p *GatewayMgr) GetByAvailableLoad() (*Gateway, bool) {
	var selected *Gateway
	p.m.Foreach(func(key string, gateway *Gateway) bool {
		if gateway == nil || gateway.Addr == "" || gateway.GrpcAddr == "" ||
			gateway.XGatewayService == nil || !gateway.Available() || gateway.AvailableLoad == 0 {
			return true
		}
		if selected == nil ||
			gateway.AvailableLoad > selected.AvailableLoad ||
			(gateway.AvailableLoad == selected.AvailableLoad && key < selected.Key) {
			cp := *gateway
			selected = &cp
		}
		return true
	})
	return selected, selected != nil
}

func (p *GatewayMgr) stopGateway(gateway *Gateway) {
	if gateway.XGatewayService == nil {
		return
	}
	gateway.Disabled()
	if err := gateway.Stop(); err != nil {
		xlog.GLog.Warnf("GatewayMgr.Stop key:%s err:%v", gateway.Key, err)
	}
}

func extractGatewayGRPCAddr(valueJson *xetcd.ValueJson) string {
	if valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
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
