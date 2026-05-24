package main

import (
	"fmt"

	gatewaycommon "server/common"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
)

// GOnlineMgr 全局 online 服务管理器
var GOnlineMgr = &OnlineMgr{
	m: xmap.NewMapMutexMgr[string, *Online](),
}

// OnlineMgr 管理所有 online 服务实例
// key: etcd key（唯一标识一个实例）
// 写操作只在 etcd actor 协程中触发（串行），读操作来自 TCP 处理协程（并发）→ MapMutexMgr
type OnlineMgr struct {
	m *xmap.MapMutexMgr[string, *Online]
}

// Add 上线：建立 gRPC 连接，启动 recvLoop，注册到 resolve，缓存实例。
// 若 key 已存在（update），先摘除旧实例再建立新实例。
func (mgr *OnlineMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := ptrStr(gs.PackageName)
	serviceName := ptrStr(gs.ServiceName)
	addr := *gs.Addr

	if old, ok := mgr.m.Find(key); ok {
		mgr.removeInfo(key, old)
	}

	connID := fmt.Sprintf("%d.%s.%d", groupID, serverName, serverID)
	online, err := newOnline(connID, addr)
	if err != nil {
		xlog.GLog.Errorf("OnlineMgr.Add dial %s failed: %v", addr, err)
		return err
	}
	online.GroupID = groupID
	online.ServerName = serverName
	online.ServerID = serverID
	online.PackageName = packageName
	online.ServiceName = serviceName

	mgr.m.Add(key, online)
	// Online 实现 IClientConn，直接注册到 resolve 哈希环
	xgrpcresolve.AddServer(groupID, serverName, serverID, online, packageName, serviceName)

	xlog.GLog.Infof("OnlineMgr.Add key=%s addr=%s total=%d", key, addr, mgr.m.Len())
	return nil
}

// Remove 下线：从 resolve 摘除，关闭流和连接。
func (mgr *OnlineMgr) Remove(key string) {
	online, ok := mgr.m.Find(key)
	if !ok {
		return
	}
	mgr.removeInfo(key, online)
	xlog.GLog.Infof("OnlineMgr.Remove key=%s total=%d", key, mgr.m.Len())
}

func (mgr *OnlineMgr) removeInfo(key string, online *Online) {
	if _, err := xgrpcresolve.RemoveServer(
		online.GroupID, online.ServerName, online.ServerID,
		online.PackageName, online.ServiceName,
	); err != nil {
		xlog.GLog.Warnf("OnlineMgr.removeInfo RemoveServer key=%s: %v", key, err)
	}
	if err := online.Stop(); err != nil {
		xlog.GLog.Warnf("OnlineMgr.removeInfo Stop key=%s: %v", key, err)
	}
	mgr.m.Del(key)
}

// GetByShardKey 通过一致性哈希选取一个 online 实例
func (mgr *OnlineMgr) GetByShardKey(shardKey string) (*Online, error) {
	iConn, err := xgrpcresolve.GetClientConnByHashRing(
		gatewaycommon.OnlinePackageName, gatewaycommon.OnlineServiceName, shardKey,
	)
	if err != nil {
		return nil, err
	}
	online, ok := iConn.(*Online)
	if !ok {
		return nil, fmt.Errorf("unexpected conn type in resolve")
	}
	return online, nil
}

// Len 返回当前在线的 online 实例数量
func (mgr *OnlineMgr) Len() int {
	return mgr.m.Len()
}
