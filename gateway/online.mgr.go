package main

import (
	"math/rand"
	"sync/atomic"

	gatewaycommon "server/common"

	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// GOnlineMgr 全局 online 服务管理器
var GOnlineMgr = &OnlineMgr{
	m: xmap.NewMapMutexMgr[string, *Online](),
}

// OnlineMgr 管理所有 online 服务实例
// 写操作只在 etcd actor 协程中触发（串行），读操作来自 TCP 处理协程（并发）→ MapMutexMgr
type OnlineMgr struct {
	m *xmap.MapMutexMgr[string, *Online] // key: etcd key
}

// Add 上线：建立 gRPC 连接，启动 recvLoop，注册到 resolve，缓存实例。
func (p *OnlineMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil ||
		valueJson.GrpcService.ServiceName == nil || valueJson.GrpcService.PackageName == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := *gs.PackageName
	serviceName := *gs.ServiceName
	addr := *gs.Addr

	online, err := newOnline(key, addr)
	if err != nil {
		xlog.GLog.Errorf("OnlineMgr.Add dial %s failed: %v", addr, err)
		return err
	}
	online.GroupID = groupID
	online.ServerName = serverName
	online.ServerID = serverID
	online.PackageName = packageName
	online.ServiceName = serviceName
	online.AvailableLoad = valueJson.AvailableLoad

	p.m.Add(key, online)
	// Online 实现 IClientConn，直接注册到 resolve
	xgrpcresolve.AddServer(groupID, serverName, serverID, online, packageName, serviceName)

	xlog.GLog.Infof("OnlineMgr.Add key:%s addr:%s total:%d", key, addr, p.m.Len())
	return nil
}

// Remove 下线：从 resolve 摘除，关闭流和连接。
func (p *OnlineMgr) Remove(key string) {
	online, ok := p.m.Find(key)
	if !ok {
		return
	}

	if _, err := xgrpcresolve.RemoveServer(online.GroupID, online.ServerName, online.ServerID, online.PackageName, online.ServiceName); err != nil {
		xlog.GLog.Errorf("OnlineMgr.Remove xgrpcresolve.RemoveServer key:%s err:%v", key, err)
		// 即使从 resolve 摘除失败，也继续关闭连接和删除缓存，避免不一致导致的请求失败
		if stopErr := online.Stop(); stopErr != nil {
			xlog.GLog.Warnf("OnlineMgr.removeInfo fallback Stop key=%s: %v", key, stopErr)
		}
	}
	p.m.Del(key)

	xlog.GLog.Infof("OnlineMgr.removeInfo RemoveServer key:%s total:%v", key, p.m.Len())
}

func (p *OnlineMgr) UpdateAvailableLoad(key string, valueJson *xetcd.ValueJson) {
	if valueJson == nil {
		return
	}
	online, ok := p.m.Find(key)
	if !ok || online == nil {
		return
	}
	atomic.StoreUint32(&online.AvailableLoad, valueJson.AvailableLoad)
	xlog.GLog.Infof("OnlineMgr.UpdateAvailableLoad key:%s availableLoad:%d", key, valueJson.AvailableLoad)
}

func (p *OnlineMgr) GetByAvailableLoad() (*Online, error) {
	var selected *Online
	var selectedLoad uint32
	p.m.Foreach(func(key string, online *Online) bool {
		if online == nil || online.XOnlineService == nil || online.GetClientConn() == nil {
			return true
		}
		availableLoad := atomic.LoadUint32(&online.AvailableLoad)
		if availableLoad == 0 {
			return true
		}
		if selected == nil || availableLoad > selectedLoad || (availableLoad == selectedLoad && key < selected.Key) {
			selected = online
			selectedLoad = availableLoad
		}
		return true
	})
	if selected == nil {
		return nil, errors.WithMessagef(xerror.Unavailable, "online available load not found %v", xruntime.Location())
	}
	return selected, nil
}

func (p *OnlineMgr) GetByRandom() (*Online, error) {
	var candidates []*Online
	p.m.Foreach(func(_ string, online *Online) bool {
		if online == nil || online.XOnlineService == nil || online.GetClientConn() == nil {
			return true
		}
		if atomic.LoadUint32(&online.AvailableLoad) == 0 {
			return true
		}
		candidates = append(candidates, online)
		return true
	})
	if len(candidates) == 0 {
		return nil, errors.WithMessagef(xerror.Unavailable, "online available load not found %v", xruntime.Location())
	}
	online := candidates[rand.Intn(len(candidates))]
	xlog.GLog.Warnf("todo menglc debug random select online key:%s availableLoad:%d", online.Key, atomic.LoadUint32(&online.AvailableLoad))
	return online, nil
}

func (p *OnlineMgr) GetForLogin() (*Online, error) {
	if xruntime.IsDebug() {
		return p.GetByRandom()
	}
	return p.GetByAvailableLoad()
}

// GetByShardKey 通过一致性哈希选取一个 online 实例
func (p *OnlineMgr) GetByShardKey(shardKey string) (*Online, error) {
	iConn, err := xgrpcresolve.GetClientConnByHashRing(gatewaycommon.OnlinePackageName, gatewaycommon.OnlineServiceName, shardKey)
	if err != nil {
		return nil, errors.WithMessagef(err, "GetByShardKey shardKey:%v failed %v", shardKey, xruntime.Location())
	}
	online, ok := iConn.(*Online)
	if !ok {
		return nil, errors.WithMessagef(err, "GetByShardKey shardKey:%v failed %v", shardKey, xruntime.Location())
	}
	return online, nil
}
