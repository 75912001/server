package main

import (
	"fmt"
	"sync"

	pb "server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Online：嵌入 *pb.XOnlineService（连接管理 + 生命周期），补充 id 和 streamMu
// ─────────────────────────────────────────────────────────────────────────────

// Online 是一个 online 服务实例。
//   - 嵌入 *pb.XOnlineService 获得 GetClientConn/Available/Disabled，
//     以及带拦截器的 dial 和 receiveLoop 错误处理。
//   - id 补充 IClientConn.GetID()（XOnlineService 无此字段）。
//   - streamMu 保护 XOnlineService 内部流字段的并发读写
//     （读：getStream；写：resetStream via Post 回调）。
type Online struct {
	*pb.XOnlineService
	id string

	streamMu sync.RWMutex // 保护 XOnlineService.GetStream/ResetStream 的并发访问

	groupID     uint32
	serverName  string
	serverID    uint32
	packageName string
	serviceName string
}

// newOnline 建立 gRPC 连接，启动 recvLoop。
// 流在 NewXOnlineService 内部同步创建，无需等待 Pre 回调。
func newOnline(id, addr string) (*Online, error) {
	xService, err := pb.NewXOnlineService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	o := &Online{id: id, XOnlineService: xService}
	_ = xService.Start()
	return o, nil
}

// GetID 补充 IClientConn 接口（XOnlineService 无此方法）
func (o *Online) GetID() string { return o.id }

// Stop 标记不可用后关闭底层流和连接
func (o *Online) Stop() error {
	o.Disabled()
	return o.XOnlineService.Stop()
}

// getStream 返回当前双向流供 Send 使用
func (o *Online) getStream() (pb.OnlineService_OnlineStreamTunnelClient, error) {
	o.streamMu.RLock()
	defer o.streamMu.RUnlock()
	s := o.GetStream()
	if s == nil {
		return nil, fmt.Errorf("online[%s] stream not ready", o.id)
	}
	return s, nil
}

// resetStream 流出错后置空，下次 getStream 返回错误（由 Post 回调或发送失败时调用）
func (o *Online) resetStream() {
	o.streamMu.Lock()
	o.ResetStream()
	o.streamMu.Unlock()
}
