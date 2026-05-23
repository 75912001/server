package main

import (
	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xlog "github.com/75912001/xlib/log"
)

// ─────────────────────────────────────────────────────────────────────────────
// etcd 回调入口：在 xlib actor 协程中串行执行，无需额外加锁
// 后续新增服务（game、chat 等）在 onServiceConnected/Disconnected 中路由分发
// ─────────────────────────────────────────────────────────────────────────────

// onEtcdAdd 新服务上线
func onEtcdAdd(args ...any) error {
	key := args[0].(string)
	valueJson := args[1].(*xetcd.ValueJson)
	return connectGrpcService(key, valueJson)
}

// onEtcdUpdate 服务信息更新
func onEtcdUpdate(args ...any) error {
	return nil
}

// onEtcdDel 服务下线
func onEtcdDel(args ...any) error {
	key := args[0].(string)
	disconnectGrpcService(key)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 通用分发（解析 etcd key，路由到对应服务的 Mgr）
// ─────────────────────────────────────────────────────────────────────────────

func connectGrpcService(key string, valueJson *xetcd.ValueJson) error {
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return nil
	}
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	err := onServiceConnected(serverName, key, valueJson)
	if err != nil {
		xlog.GLog.Errorf("connectGrpcService serverName=%s key=%s: %v", serverName, key, err)
	}
	return err
}

func disconnectGrpcService(key string) {
	_, _, serverName, _ := xetcd.Parse(key)
	onServiceDisconnected(serverName, key)
}

// ─────────────────────────────────────────────────────────────────────────────
// 工具
// ─────────────────────────────────────────────────────────────────────────────

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
