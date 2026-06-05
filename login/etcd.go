package main

import (
	"server/common"

	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// onEtcdAdd 处理 cache/gateway 服务注册事件。
func onEtcdAdd(args ...any) error {
	key, valueJson, ok := parseEtcdCallbackArgs(args...)
	if !ok || valueJson == nil {
		return nil
	}
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	switch serverName {
	case common.CacheServerName:
		GCacheMgr.Remove(key)
		if err := GCacheMgr.Add(key, valueJson); err != nil {
			return errors.WithMessagef(err, "add cache key:%s %v", key, xruntime.Location())
		}
	case common.GatewayServerName:
		GGatewayMgr.Remove(key)
		if err := GGatewayMgr.Add(key, valueJson); err != nil {
			return errors.WithMessagef(err, "add gateway key:%s %v", key, xruntime.Location())
		}
	}
	return nil
}

// onEtcdUpdate 处理服务信息更新事件；login 只关心 gateway 负载和地址变化。
func onEtcdUpdate(args ...any) error {
	key, valueJson, ok := parseEtcdCallbackArgs(args...)
	if !ok || valueJson == nil {
		return nil
	}
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	switch serverName {
	case common.GatewayServerName:
		if err := GGatewayMgr.Update(key, valueJson); err != nil {
			return errors.WithMessagef(err, "update gateway key:%s %v", key, xruntime.Location())
		}
	}
	return nil
}

// onEtcdDel 处理 cache/gateway 服务下线事件。
func onEtcdDel(args ...any) error {
	if len(args) == 0 {
		return nil
	}
	key, ok := args[0].(string)
	if !ok {
		return nil
	}
	_, _, serverName, _ := xetcd.Parse(key)
	switch serverName {
	case common.CacheServerName:
		GCacheMgr.Remove(key)
	case common.GatewayServerName:
		GGatewayMgr.Remove(key)
	}
	return nil
}

// parseEtcdCallbackArgs 从 xlib etcd 回调参数中解析 key 和 ValueJson。
func parseEtcdCallbackArgs(args ...any) (string, *xetcd.ValueJson, bool) {
	if len(args) < 2 {
		return "", nil, false
	}
	key, ok := args[0].(string)
	if !ok {
		return "", nil, false
	}
	valueJson, ok := args[1].(*xetcd.ValueJson)
	return key, valueJson, ok
}
