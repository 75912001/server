package main

import (
	"server/common"

	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

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
