package main

import (
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
)

// GUserMgr 全局用户管理器。
//
//	按 IRemote 索引（接口值含 *tcp.Remote 指针，可比较），方便 TCP 回调里通过 remote 反查到 *User。
var GUserMgr = &UserMgr{
	byRemote: xmap.NewMapMutexMgr[xnetcommon.IRemote, *User](),
	byUID:    xmap.NewMapMutexMgr[uint64, *User](),
}

// UserMgr 管理所有已连接的用户。
type UserMgr struct {
	byRemote *xmap.MapMutexMgr[xnetcommon.IRemote, *User]
	byUID    *xmap.MapMutexMgr[uint64, *User]
}

// Add 创建用户并登记（TCP OnConnect 触发）。
func (m *UserMgr) Add(remote xnetcommon.IRemote) *User {
	u := newUser(remote)
	m.byRemote.Add(remote, u)
	return u
}

// Get 查找用户（TCP OnPacket 用 remote 反查）。
func (m *UserMgr) Get(remote xnetcommon.IRemote) *User {
	u, _ := m.byRemote.Find(remote)
	return u
}

func (m *UserMgr) GetByUID(uid uint64) *User {
	u, _ := m.byUID.Find(uid)
	return u
}

func (m *UserMgr) BindUID(uid uint64, u *User) {
	if old := m.GetByUID(uid); old != nil && old != u {
		xlog.GLog.Infof("duplicate uid login, disconnect old user")
		old.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
	}
	m.byUID.Add(uid, u)
}

func (m *UserMgr) PostOnlineFrame(frameUID uint64, frame *pb.OnlineTunnelFrame) {
	user := m.GetByUID(frameUID)
	if user == nil {
		xlog.GLog.Errorf("online frame uid=%d not found", frameUID)
		return
	}
	user.PostFrame(frame)
}

// Remove 摘除 remote 对应用户，同步执行 Cleanup，并移除 uid 索引。
func (m *UserMgr) Remove(remote xnetcommon.IRemote) *User {
	u, ok := m.byRemote.Find(remote)
	if !ok {
		return nil
	}
	m.byRemote.Del(remote)

	m.byUID.Del(u.uid)

	u.PostSyncCleanup(remote.GetDisconnectReason())

	u.remote.Stop()
	return u
}

// Len 当前在线用户数。
func (m *UserMgr) Len() int { return m.byRemote.Len() }
