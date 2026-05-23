package main

import (
	"fmt"

	xetcd "github.com/75912001/xlib/etcd"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xgrpcutil "github.com/75912001/xlib/grpc/util"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GOnlineMgr 全局 online 服务管理器
var GOnlineMgr = &OnlineMgr{
	m: xmap.NewMapMutexMgr[string, *Online](),
}

// Online 记录一条 online 实例的连接信息
type Online struct {
	conn        *grpcClientConn
	groupID     uint32
	serverName  string
	serverID    uint32
	packageName string
	serviceName string
}

// OnlineMgr 管理所有 online 服务实例的 gRPC 连接
// key: etcd key（唯一标识一个服务实例）
// 写操作只在 etcd actor 协程中触发（串行），读操作来自 TCP 处理协程（并发），
// 因此使用 MapMutexMgr 保证并发安全。
type OnlineMgr struct {
	m *xmap.MapMutexMgr[string, *Online]
}

// Add 上线：建立 gRPC 连接并注册到 resolve。
// 若该 key 已存在（update），先摘除旧连接再建立新连接。
func (mgr *OnlineMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return nil
	}
	_, groupID, serverName, serverID := xetcd.Parse(key)
	gs := valueJson.GrpcService
	packageName := ptrStr(gs.PackageName)
	serviceName := ptrStr(gs.ServiceName)
	addr := *gs.Addr

	// 若已存在旧连接则先摘除
	if old, ok := mgr.m.Find(key); ok {
		mgr.removeInfo(key, old)
	}

	connID := fmt.Sprintf("%d.%s.%d", groupID, serverName, serverID)

	clientConn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		xlog.GLog.Errorf("OnlineMgr.Add dial %s failed: %v", addr, err)
		return err
	}
	conn := &grpcClientConn{id: connID, conn: clientConn, available: true}

	info := &Online{
		conn:        conn,
		groupID:     groupID,
		serverName:  serverName,
		serverID:    serverID,
		packageName: packageName,
		serviceName: serviceName,
	}
	mgr.m.Add(key, info)
	xgrpcresolve.AddServer(groupID, serverName, serverID, conn, packageName, serviceName)

	xlog.GLog.Infof("OnlineMgr.Add key=%s addr=%s total=%d", key, addr, mgr.m.Len())
	return nil
}

// Remove 下线：从 resolve 摘除连接并关闭。
func (mgr *OnlineMgr) Remove(key string) {
	info, ok := mgr.m.Find(key)
	if !ok {
		return
	}
	mgr.removeInfo(key, info)
	xlog.GLog.Infof("OnlineMgr.Remove key=%s total=%d", key, mgr.m.Len())
}

func (mgr *OnlineMgr) removeInfo(key string, info *Online) {
	if _, err := xgrpcresolve.RemoveServer(
		info.groupID, info.serverName, info.serverID,
		info.packageName, info.serviceName,
	); err != nil {
		xlog.GLog.Warnf("OnlineMgr.removeInfo RemoveServer key=%s: %v", key, err)
	}
	if err := info.conn.Stop(); err != nil {
		xlog.GLog.Warnf("OnlineMgr.removeInfo Stop conn key=%s: %v", key, err)
	}
	mgr.m.Del(key)
}

// GetConn 通过一致性哈希获取一条到 online 的连接
func (mgr *OnlineMgr) GetConn(shardKey string) (xgrpcutil.IClientConn, error) {
	return xgrpcresolve.GetClientConnByHashRing(onlinePackageName, onlineServiceName, shardKey)
}

// Len 返回当前在线的 online 实例数量
func (mgr *OnlineMgr) Len() int {
	return mgr.m.Len()
}
