package main

import (
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var GGatewayPeerMgr = &GatewayPeerMgr{
	m: xmap.NewMapMutexMgr[string, *GatewayPeer](),
}

type GatewayPeerMgr struct {
	m *xmap.MapMutexMgr[string, *GatewayPeer]
}

type GatewayPeer struct {
	*pb.XGatewayService
	Key string

	GroupID     uint32
	ServerName  string
	ServerID    uint32
	PackageName string
	ServiceName string
}

func newGatewayPeer(key string, addr string) (*GatewayPeer, error) {
	xService, err := pb.NewXGatewayService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	return &GatewayPeer{Key: key, XGatewayService: xService}, nil
}

func (p *GatewayPeerMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	peer, err := newGatewayPeer(key, *gs.Addr)
	if err != nil {
		xlog.GLog.Errorf("GatewayPeerMgr.Add dial %s failed: %v", *gs.Addr, err)
		return err
	}
	peer.GroupID = groupID
	peer.ServerName = serverName
	peer.ServerID = serverID
	peer.PackageName = *gs.PackageName
	peer.ServiceName = *gs.ServiceName

	p.m.Add(key, peer)
	xgrpcresolve.AddServer(groupID, serverName, serverID, peer, peer.PackageName, peer.ServiceName)

	xlog.GLog.Infof("GatewayPeerMgr.Add key:%s addr:%s total:%d", key, *gs.Addr, p.m.Len())
	return nil
}

func (p *GatewayPeerMgr) Remove(key string) {
	peer, ok := p.m.Find(key)
	if !ok {
		return
	}
	if _, err := xgrpcresolve.RemoveServer(peer.GroupID, peer.ServerName, peer.ServerID, peer.PackageName, peer.ServiceName); err != nil {
		xlog.GLog.Warnf("GatewayPeerMgr.Remove RemoveServer key:%s err:%v", key, err)
	}
	if err := peer.Stop(); err != nil {
		xlog.GLog.Warnf("GatewayPeerMgr.Remove Stop key:%s err:%v", key, err)
	}
	p.m.Del(key)
	xlog.GLog.Infof("GatewayPeerMgr.Remove key:%s total:%d", key, p.m.Len())
}

func (p *GatewayPeerMgr) Get(key string) *GatewayPeer {
	peer, _ := p.m.Find(key)
	return peer
}

func (p *GatewayPeer) Client() (pb.GatewayServiceClient, error) {
	if p == nil {
		return nil, errors.WithMessagef(xerror.NotFound, "GatewayPeer nil %v", xruntime.Location())
	}
	if p.XGatewayService == nil {
		return nil, errors.WithMessagef(xerror.NotFound, "GatewayPeer %v %v", p.Key, xruntime.Location())
	}
	return pb.NewGatewayServiceClient(p.GetClientConn()), nil
}

func (p *GatewayPeer) GetID() string { return p.Key }

func (p *GatewayPeer) GetClientConn() *grpc.ClientConn {
	if p.XGatewayService == nil {
		return nil
	}
	return p.XGatewayService.GetClientConn()
}
