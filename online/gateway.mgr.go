package main

import (
	xmap "github.com/75912001/xlib/map"

	pb "server/proto/pb"
)

var GGatewayMgr = &GatewayMgr{
	m: xmap.NewMapMutexMgr[string, *Gateway](), // key: gateway etcd key
}

type GatewayMgr struct {
	m *xmap.MapMutexMgr[string, *Gateway]
}

func (p *GatewayMgr) Get(id string) *Gateway {
	gateway, _ := p.m.Find(id)
	return gateway
}

func (p *GatewayMgr) BindStream(id string, stream pb.OnlineService_OnlineStreamTunnelServer) *Gateway {
	gateway := p.Get(id)
	if gateway == nil {
		gateway = newGateway(id)
		p.m.Add(id, gateway)
	}
	gateway.BindStream(stream)
	return gateway
}

func (p *GatewayMgr) ResetStream(id string, stream pb.OnlineService_OnlineStreamTunnelServer) {
	if gateway := p.Get(id); gateway != nil {
		gateway.ResetStream(stream)
	}
}

func (p *GatewayMgr) Len() int {
	return p.m.Len()
}
