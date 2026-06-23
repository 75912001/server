package main

import (
	"context"
	"math/rand"
	"sync"
	"time"

	common "server/common"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type discoveredCache struct {
	*pb.XCacheService
	key         string
	addr        string
	groupID     uint32
	serverName  string
	serverID    uint32
	packageName string
	serviceName string
}

func (p *discoveredCache) GetID() string { return p.key }

type discoveredGateway struct {
	*pb.XGatewayService
	key           string
	tcpAddr       string
	grpcAddr      string
	groupID       uint32
	serverName    string
	serverID      uint32
	availableLoad uint32
}

func (p *discoveredGateway) GetID() string { return p.key }

var GDiscoveredCacheMgr = &discoveredCacheMgr{
	m: xmap.NewMapMutexMgr[string, *discoveredCache](),
}

var (
	discoveredGatewayMu   sync.Mutex
	discoveredGatewayMap  = make(map[string]*discoveredGateway)
	discoveredGatewayKey  string
	discoveredGatewayChan = make(chan struct{}, 1)
)

type discoveredCacheMgr struct {
	m *xmap.MapMutexMgr[string, *discoveredCache]
}

func startServiceDiscovery(ctx context.Context) error {
	if len(GConfigYaml.Etcd.Endpoints) == 0 {
		return errors.WithMessage(xerror.Config, "etcd endpoints is empty")
	}
	if GRobotManager == nil || GRobotManager.iEventMgr == nil {
		return errors.WithMessage(xerror.Config, "robot manager is nil")
	}
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()
	xetcd.GEtcd = xetcd.NewEtcd(xetcd.NewOptions().
		WithEndpoints(GConfigYaml.Etcd.Endpoints).
		WithTTL(GConfigYaml.Etcd.TTLDuration).
		WithWatchKeyPrefix(xetcd.GenPrefixKey(GConfigYaml.Etcd.ProjectName)).
		WithIOut(GRobotManager.iEventMgr).
		WithAddCallback(xcontrol.NewCallBack(onServiceEtcdAdd)).
		WithUpdateCallback(xcontrol.NewCallBack(onServiceEtcdUpdate)).
		WithDelCallback(xcontrol.NewCallBack(onServiceEtcdDel)))
	if xetcd.GEtcd == nil {
		return errors.WithMessage(xerror.Config, "new etcd failed")
	}
	if err := xetcd.GEtcd.Start(ctx, ""); err != nil {
		return errors.WithMessage(err, "start service discovery failed")
	}
	return nil
}

func stopServiceDiscovery() error {
	var err error
	if xetcd.GEtcd != nil {
		err = xetcd.GEtcd.Stop()
		xetcd.GEtcd = nil
	}
	closeDiscoveredGateways()
	GDiscoveredCacheMgr.closeAll()
	return err
}

func onServiceEtcdAdd(args ...any) error {
	key := args[0].(string)
	valueJson := args[1].(*xetcd.ValueJson)
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	switch serverName {
	case common.CacheServerName:
		if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
			valueJson.GrpcService.PackageName == nil || valueJson.GrpcService.ServiceName == nil {
			return nil
		}
		return GDiscoveredCacheMgr.add(key, valueJson)
	case common.GatewayServerName:
		return updateDiscoveredGateway(key, valueJson)
	default:
		return nil
	}
}

func onServiceEtcdUpdate(args ...any) error {
	key := args[0].(string)
	valueJson := args[1].(*xetcd.ValueJson)
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	switch serverName {
	case common.GatewayServerName:
		return updateDiscoveredGateway(key, valueJson)
	default:
		return nil
	}
}

func onServiceEtcdDel(args ...any) error {
	key := args[0].(string)
	_, _, serverName, _ := xetcd.Parse(key)
	switch serverName {
	case common.CacheServerName:
		GDiscoveredCacheMgr.remove(key)
	case common.GatewayServerName:
		clearDiscoveredGateway(key)
	}
	return nil
}

func updateDiscoveredGateway(key string, valueJson *xetcd.ValueJson) error {
	gateway, err := newDiscoveredGateway(key, valueJson)
	if err != nil {
		return err
	}
	if gateway == nil {
		return nil
	}
	discoveredGatewayMu.Lock()
	old := discoveredGatewayMap[key]
	if old != nil && old.tcpAddr == gateway.tcpAddr && old.grpcAddr == gateway.grpcAddr {
		old.availableLoad = gateway.availableLoad
		discoveredGatewayMu.Unlock()
		_ = gateway.Stop()
		return nil
	}
	discoveredGatewayMap[key] = gateway
	discoveredGatewayKey = key
	discoveredGatewayMu.Unlock()
	select {
	case discoveredGatewayChan <- struct{}{}:
	default:
	}
	if old != nil {
		_ = old.Stop()
	}
	ColorPrintf(Cyan, "gateway discovered key=%s tcp=%s grpc=%s\n", key, gateway.tcpAddr, gateway.grpcAddr)
	return nil
}

func clearDiscoveredGateway(key string) {
	discoveredGatewayMu.Lock()
	old := discoveredGatewayMap[key]
	delete(discoveredGatewayMap, key)
	discoveredGatewayKey = ""
	for candidateKey := range discoveredGatewayMap {
		discoveredGatewayKey = candidateKey
		break
	}
	discoveredGatewayMu.Unlock()
	if old != nil {
		_ = old.Stop()
	}
	ColorPrintf(Yellow, "gateway removed key=%s\n", key)
}

func waitGatewayAddr(timeout time.Duration) (string, error) {
	gateway, err := waitGateway(timeout)
	if err != nil {
		return "", err
	}
	return gateway.tcpAddr, nil
}

func waitGateway(timeout time.Duration) (*discoveredGateway, error) {
	discoveredGatewayMu.Lock()
	gateway := selectDiscoveredGatewayLocked()
	discoveredGatewayMu.Unlock()
	if gateway != nil {
		return gateway, nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-discoveredGatewayChan:
		discoveredGatewayMu.Lock()
		gateway = selectDiscoveredGatewayLocked()
		discoveredGatewayMu.Unlock()
		if gateway == nil {
			return nil, errors.WithMessage(xerror.NotFound, "gateway addr is empty")
		}
		return gateway, nil
	case <-timer.C:
		return nil, errors.WithMessage(xerror.Timeout, "wait gateway addr timeout")
	}
}

func selectDiscoveredGatewayLocked() *discoveredGateway {
	if len(discoveredGatewayMap) == 0 {
		return nil
	}
	if xruntime.IsDebug() {
		target := rand.Intn(len(discoveredGatewayMap))
		index := 0
		for _, gateway := range discoveredGatewayMap {
			if index == target {
				return gateway
			}
			index++
		}
	}
	if discoveredGatewayKey != "" {
		return discoveredGatewayMap[discoveredGatewayKey]
	}
	for _, gateway := range discoveredGatewayMap {
		return gateway
	}
	return nil
}

func gatewayTCPAddr(valueJson *xetcd.ValueJson) string {
	if valueJson == nil {
		return ""
	}
	for _, serverNet := range valueJson.ServerNet {
		if serverNet == nil || serverNet.Addr == nil {
			continue
		}
		if serverNet.Type == nil || *serverNet.Type == xnetcommon.ServerNetTypeNameTCP {
			return *serverNet.Addr
		}
	}
	return ""
}

func gatewayGRPCAddr(valueJson *xetcd.ValueJson) string {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return ""
	}
	return *valueJson.GrpcService.Addr
}

func newDiscoveredGateway(key string, valueJson *xetcd.ValueJson) (*discoveredGateway, error) {
	tcpAddr := gatewayTCPAddr(valueJson)
	grpcAddr := gatewayGRPCAddr(valueJson)
	if tcpAddr == "" || grpcAddr == "" {
		return nil, nil
	}
	xService, err := pb.NewXGatewayService(grpcAddr)
	if err != nil {
		return nil, errors.WithMessagef(err, "new gateway service key:%s addr:%s", key, grpcAddr)
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	return &discoveredGateway{
		XGatewayService: xService,
		key:             key,
		tcpAddr:         tcpAddr,
		grpcAddr:        grpcAddr,
		groupID:         groupID,
		serverName:      serverName,
		serverID:        serverID,
		availableLoad:   valueJson.AvailableLoad,
	}, nil
}

func closeDiscoveredGateways() {
	discoveredGatewayMu.Lock()
	gateways := discoveredGatewayMap
	discoveredGatewayMap = make(map[string]*discoveredGateway)
	discoveredGatewayKey = ""
	discoveredGatewayMu.Unlock()
	for _, gateway := range gateways {
		if gateway != nil {
			_ = gateway.Stop()
		}
	}
}

func (p *discoveredCacheMgr) add(key string, valueJson *xetcd.ValueJson) error {
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	if cache, ok := p.m.Find(key); ok {
		if cache.addr == *gs.Addr && cache.packageName == *gs.PackageName && cache.serviceName == *gs.ServiceName {
			return nil
		}
		p.remove(key)
	}
	xService, err := pb.NewXCacheService(*gs.Addr)
	if err != nil {
		return errors.WithMessagef(err, "new cache service key:%s addr:%s", key, *gs.Addr)
	}
	cache := &discoveredCache{
		XCacheService: xService,
		key:           key,
		addr:          *gs.Addr,
		groupID:       groupID,
		serverName:    serverName,
		serverID:      serverID,
		packageName:   *gs.PackageName,
		serviceName:   *gs.ServiceName,
	}
	p.m.Add(key, cache)
	xgrpcresolve.AddServer(cache.groupID, cache.serverName, cache.serverID, cache, cache.packageName, cache.serviceName)
	ColorPrintf(Cyan, "cache discovered key=%s addr=%s\n", key, *gs.Addr)
	return nil
}

func (p *discoveredCacheMgr) remove(key string) {
	cache, ok := p.m.Find(key)
	if !ok {
		return
	}
	if _, err := xgrpcresolve.RemoveServer(cache.groupID, cache.serverName, cache.serverID, cache.packageName, cache.serviceName); err != nil {
		_ = cache.Stop()
	}
	p.m.Del(key)
	ColorPrintf(Yellow, "cache removed key=%s\n", key)
}

func (p *discoveredCacheMgr) closeAll() {
	var keys []string
	p.m.Foreach(func(key string, value *discoveredCache) bool {
		keys = append(keys, key)
		return true
	})
	for _, key := range keys {
		p.remove(key)
	}
}
