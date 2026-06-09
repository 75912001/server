package main

import (
	"sync/atomic"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// GGatewayMgr 管理 login 发现到的 gateway 节点，用于按 availableLoad 分配客户端入口。
var GGatewayMgr = newGatewayMgr()

// GatewayMgr 保存 gateway 节点连接；节点 map 由 xlib MapMutexMgr 保护，AvailableLoad 使用 atomic 维护本地估算值。
type GatewayMgr struct {
	m *xmap.MapMutexMgr[string, *Gateway] // key: etcd server key
}

// Gateway 是 login 侧缓存的 gateway 节点连接和负载信息。
type Gateway struct {
	*pb.XGatewayService

	Key           string // gateway 在 etcd 中的 server key
	Addr          string // 客户端连接 gateway 的 TCP 地址
	GrpcAddr      string // login 直连 gateway unary 的 gRPC 地址
	GroupID       uint32 // etcd 分组 ID
	ServerName    string // 服务名
	ServerID      uint32 // 服务实例 ID
	AvailableLoad uint32 // gateway 本地估算可用负载，etcd 更新会覆盖
}

// newGatewayMgr 创建 gateway 节点管理器。
func newGatewayMgr() *GatewayMgr {
	return &GatewayMgr{
		m: xmap.NewMapMutexMgr[string, *Gateway](),
	}
}

// Add 新增 gateway 节点；add 事件会先 remove 旧节点，再创建新连接。
func (p *GatewayMgr) Add(key string, valueJson *xetcd.ValueJson) error {
	_, groupID, serverName, serverID := xetcd.Parse(key)
	addr := extractGatewayAddr(valueJson)
	grpcAddr := extractGatewayGRPCAddr(valueJson)
	if grpcAddr == "" {
		return errors.Errorf("gateway grpc addr is empty key:%s %v", key, xruntime.Location())
	}

	xGatewayService, err := pb.NewXGatewayService(grpcAddr)
	if err != nil {
		return errors.WithMessagef(err, "new gateway service key:%s addr:%s %v", key, grpcAddr, xruntime.Location())
	}

	gateway := &Gateway{
		XGatewayService: xGatewayService,
		Key:             key,
		Addr:            addr,
		GrpcAddr:        grpcAddr,
		GroupID:         groupID,
		ServerName:      serverName,
		ServerID:        serverID,
		AvailableLoad:   valueJson.AvailableLoad,
	}

	p.m.Add(key, gateway)
	total := p.m.Len()

	xlog.GLog.Infof("GatewayMgr.Add key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, gateway.Addr, gateway.GrpcAddr, gateway.AvailableLoad, total)
	return nil
}

// Update 更新 gateway 节点负载和地址；gRPC 地址未变化时复用旧连接。
func (p *GatewayMgr) Update(key string, valueJson *xetcd.ValueJson) error {
	_, groupID, serverName, serverID := xetcd.Parse(key)
	addr := extractGatewayAddr(valueJson)
	grpcAddr := extractGatewayGRPCAddr(valueJson)
	if grpcAddr == "" {
		return errors.Errorf("gateway grpc addr is empty key:%s %v", key, xruntime.Location())
	}

	old, _ := p.m.Find(key)
	if old != nil && old.GrpcAddr == grpcAddr && old.XGatewayService != nil {
		old.Addr = addr
		old.GroupID = groupID
		old.ServerName = serverName
		old.ServerID = serverID
		atomic.StoreUint32(&old.AvailableLoad, valueJson.AvailableLoad)
		total := p.m.Len()

		xlog.GLog.Infof("GatewayMgr.Update reuse key:%s addr:%s grpcAddr:%s availableLoad:%d total:%d", key, old.Addr, old.GrpcAddr, valueJson.AvailableLoad, total)
		return nil
	}

	return p.Add(key, valueJson)
}

// Remove 移除 gateway 节点并关闭对应 gRPC 连接。
func (p *GatewayMgr) Remove(key string) {
	gateway, ok := p.m.Find(key)
	if !ok {
		return
	}
	p.m.Del(key)
	total := p.m.Len()

	p.stopGateway(gateway)
	xlog.GLog.Infof("GatewayMgr.Remove key:%s total:%d", key, total)
}

// StopAll 停止当前 login 已发现的所有 gateway 连接。
func (p *GatewayMgr) StopAll() {
	keys := make([]string, 0, p.m.Len())
	p.m.Foreach(func(key string, _ *Gateway) bool {
		keys = append(keys, key)
		return true
	})

	for _, key := range keys {
		p.Remove(key)
	}
}

// ReserveByAvailableLoad 选择本地 availableLoad 最大的 gateway，并先扣减本地负载；etcd 更新会覆盖本地负载。
func (p *GatewayMgr) ReserveByAvailableLoad() (*Gateway, bool) {
	for {
		selected, selectedLoad := p.selectByAvailableLoad()
		if selected == nil {
			return nil, false
		}
		if !reserveAvailableLoad(&selected.AvailableLoad, selectedLoad) {
			continue
		}

		cp := *selected
		cp.AvailableLoad = selectedLoad - 1
		return &cp, true
	}
}

func (p *GatewayMgr) selectByAvailableLoad() (*Gateway, uint32) {
	var selected *Gateway
	var selectedLoad uint32
	p.m.Foreach(func(key string, gateway *Gateway) bool {
		if gateway == nil || gateway.Addr == "" || gateway.GrpcAddr == "" ||
			gateway.XGatewayService == nil || !gateway.Available() {
			return true
		}
		availableLoad := atomic.LoadUint32(&gateway.AvailableLoad)
		if availableLoad == 0 {
			return true
		}
		if selected == nil ||
			availableLoad > selectedLoad ||
			(availableLoad == selectedLoad && key < selected.Key) {
			selected = gateway
			selectedLoad = availableLoad
		}
		return true
	})
	return selected, selectedLoad
}

func reserveAvailableLoad(load *uint32, expected uint32) bool {
	if expected == 0 {
		return false
	}
	return atomic.CompareAndSwapUint32(load, expected, expected-1)
}

// stopGateway 将 gateway 标记为不可用并关闭 gRPC 连接。
func (p *GatewayMgr) stopGateway(gateway *Gateway) {
	if gateway.XGatewayService == nil {
		return
	}
	gateway.Disabled()
	if err := gateway.Stop(); err != nil {
		xlog.GLog.Warnf("GatewayMgr.Stop key:%s err:%v", gateway.Key, err)
	}
}

// extractGatewayGRPCAddr 从 etcd server 信息中读取 gateway gRPC 地址。
func extractGatewayGRPCAddr(valueJson *xetcd.ValueJson) string {
	if valueJson.GrpcService == nil || valueJson.GrpcService.Addr == nil {
		return ""
	}
	return *valueJson.GrpcService.Addr
}

// extractGatewayAddr 从 etcd server 网络信息中读取客户端 TCP 连接地址。
func extractGatewayAddr(valueJson *xetcd.ValueJson) string {
	for _, serverNet := range valueJson.ServerNet {
		if serverNet == nil || serverNet.Addr == nil || *serverNet.Addr == "" {
			continue
		}
		netType := xnetcommon.ServerNetTypeNameTCP
		if serverNet.Type != nil {
			netType = *serverNet.Type
		}
		if netType == xnetcommon.ServerNetTypeNameTCP {
			return *serverNet.Addr
		}
	}
	return ""
}
