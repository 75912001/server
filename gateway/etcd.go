package main

import (
	common "server/common"

	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xlog "github.com/75912001/xlib/log"
)

// onEtcdAdd 新服务上线
func onEtcdAdd(args ...any) error {
	key := args[0].(string)
	valueJson := args[1].(*xetcd.ValueJson)
	if valueJson == nil || valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return nil
	}
	msgType, _, serverName, _ := xetcd.Parse(key)
	if msgType != xetcdconstants.WatchMsgTypeServer {
		return nil
	}
	switch serverName {
	case common.OnlineServerName:
		if err := GOnlineMgr.Add(key, valueJson); err != nil {
			xlog.GLog.Fatalf("onEtcdAdd key=%s: %v", key, err)
			return err
		}
		xlog.GLog.Infof("onEtcdAdd key:%s", key)
	}
	return nil
}

// onEtcdUpdate 服务信息更新
func onEtcdUpdate(args ...any) error {
	return nil
}

// onEtcdDel 服务下线
func onEtcdDel(args ...any) error {
	key := args[0].(string)
	_, _, serverName, _ := xetcd.Parse(key)
	switch serverName {
	case common.OnlineServerName:
		GOnlineMgr.Remove(key)
		xlog.GLog.Infof("onEtcdDel key:%s", key)
	}
	return nil
}
