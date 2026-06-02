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

var GDiscoveredCacheMgr = &discoveredCacheMgr{
	m: xmap.NewMapMutexMgr[string, *discoveredCache](),
}

var (
	discoveredGatewayMu   sync.Mutex
	discoveredGatewayMap  = make(map[string]string)
	discoveredGatewayAddr string
	discoveredGatewayChan = make(chan string, 1)
)

type discoveredCacheMgr struct {
	m *xmap.MapMutexMgr[string, *discoveredCache]
}

func startServiceDiscovery(ctx context.Context) error {
	if len(GConfigYaml.Etcd.Endpoints) == 0 {
		return errors.WithMessage(xerror.Config, "etcd endpoints is empty")
	}
	xgrpcprotoregistry.Init()
	xgrpcselector.Init()
	xetcd.GEtcd = xetcd.NewEtcd(xetcd.NewOptions().
		WithEndpoints(GConfigYaml.Etcd.Endpoints).
		WithTTL(GConfigYaml.Etcd.TTLDuration).
		WithWatchKeyPrefix(xetcd.GenPrefixKey(GConfigYaml.Etcd.ProjectName)).
		WithIOut(GetClient().iEventMgr).
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
	return nil
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
	addr := gatewayTCPAddr(valueJson)
	if addr == "" {
		return nil
	}
	discoveredGatewayMu.Lock()
	if discoveredGatewayMap[key] == addr {
		discoveredGatewayMu.Unlock()
		return nil
	}
	discoveredGatewayMap[key] = addr
	discoveredGatewayAddr = addr
	discoveredGatewayMu.Unlock()
	select {
	case discoveredGatewayChan <- addr:
	default:
	}
	//	ColorPrintf(Cyan, "gateway discovered key=%s addr=%s\n", key, addr)
	return nil
}

func clearDiscoveredGateway(key string) {
	discoveredGatewayMu.Lock()
	delete(discoveredGatewayMap, key)
	discoveredGatewayAddr = ""
	for _, addr := range discoveredGatewayMap {
		discoveredGatewayAddr = addr
		break
	}
	discoveredGatewayMu.Unlock()
	ColorPrintf(Yellow, "gateway removed key=%s\n", key)
}

func waitGatewayAddr(timeout time.Duration) (string, error) {
	discoveredGatewayMu.Lock()
	addr := selectDiscoveredGatewayAddrLocked()
	discoveredGatewayMu.Unlock()
	if addr != "" {
		return addr, nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-discoveredGatewayChan:
		discoveredGatewayMu.Lock()
		addr = selectDiscoveredGatewayAddrLocked()
		discoveredGatewayMu.Unlock()
		if addr == "" {
			return "", errors.WithMessage(xerror.NotFound, "gateway addr is empty")
		}
		return addr, nil
	case <-timer.C:
		return "", errors.WithMessage(xerror.Timeout, "wait gateway addr timeout")
	}
}

func selectDiscoveredGatewayAddrLocked() string {
	if len(discoveredGatewayMap) == 0 {
		return ""
	}
	if xruntime.IsDebug() {
		target := rand.Intn(len(discoveredGatewayMap))
		index := 0
		for key, addr := range discoveredGatewayMap {
			if index == target {
				ColorPrintf(Yellow, "todo menglc debug random select gateway key=%s addr=%s\n", key, addr)
				if log != nil {
					log.Warnf("todo menglc debug random select gateway key=%s addr=%s", key, addr)
				}
				return addr
			}
			index++
		}
	}
	if discoveredGatewayAddr != "" {
		return discoveredGatewayAddr
	}
	for _, addr := range discoveredGatewayMap {
		return addr
	}
	return ""
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
