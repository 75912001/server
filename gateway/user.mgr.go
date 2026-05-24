package main

import (
	pb "server/proto/pb"

	"sync/atomic"

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
	shards   []*UserShard
	next     atomic.Uint64
}

// Init 初始化用户分片 actor；必须在任何 newUser 调用之前完成。
func (m *UserMgr) Init() {
	if len(m.shards) > 0 {
		return
	}
	count := cfgUserActorCount()
	if count <= 0 {
		count = int(GatewayDefaultUserActorCount)
	}
	m.shards = make([]*UserShard, 0, count)
	for i := 0; i < count; i++ {
		m.shards = append(m.shards, newUserShard(i))
	}
}

// Add 创建用户并登记（TCP OnConnect 触发）。
func (m *UserMgr) Add(remote xnetcommon.IRemote) *User {
	u := newUser(remote, m.nextShard())
	m.byRemote.Add(remote, u)
	return u
}

func (m *UserMgr) nextShard() *UserShard {
	idx := m.next.Add(1) - 1
	return m.shards[int(idx%uint64(len(m.shards)))]
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
	m.byUID.Add(uid, u)
}

func (m *UserMgr) PostOnlineFrame(frameUID uint64, frame *pb.OnlineTunnelFrame) {
	user := m.GetByUID(frameUID)
	if user == nil {
		xlog.PrintfErr("online frame uid=%d not found", frameUID)
		return
	}
	user.shard.PostFrame(frame)
}

// Remove 摘除并返回用户（TCP OnDisconnect 触发，调用方负责 Cleanup）。
func (m *UserMgr) Remove(remote xnetcommon.IRemote) *User {
	u, ok := m.byRemote.Find(remote)
	if !ok {
		return nil
	}
	m.byRemote.Del(remote)
	if err := u.shard.PostCleanup(u); err != nil {
		return u
	}
	if uid := u.uid.Load(); uid != 0 {
		m.byUID.Del(uid)
	}
	return u
}

// Len 当前在线用户数。
func (m *UserMgr) Len() int { return m.byRemote.Len() }
