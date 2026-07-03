package main

import (
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xnetcommon "github.com/75912001/xlib/net/common"
)

// GAccountMgr 全局用户管理器。
//
//	按 IRemote 索引（接口值含 *tcp.Remote 指针，可比较），方便 TCP 回调里通过 remote 反查到 *Account。
var GAccountMgr = &AccountMgr{
	byRemote: xmap.NewMapMutexMgr[xnetcommon.IRemote, *Account](),
	byAID:    xmap.NewMapMutexMgr[uint64, *Account](),
}

// AccountMgr 管理所有已连接的用户。
type AccountMgr struct {
	byRemote *xmap.MapMutexMgr[xnetcommon.IRemote, *Account]
	byAID    *xmap.MapMutexMgr[uint64, *Account]
}

// Add 创建用户并登记（TCP OnConnect 触发）。
func (p *AccountMgr) Add(remote xnetcommon.IRemote) *Account {
	u := newAccount(remote)
	p.byRemote.Add(remote, u)
	return u
}

// Get 查找用户（TCP OnPacket 用 remote 反查）。
func (p *AccountMgr) Get(remote xnetcommon.IRemote) *Account {
	u, _ := p.byRemote.Find(remote)
	return u
}

func (p *AccountMgr) GetByAID(aid uint64) *Account {
	u, _ := p.byAID.Find(aid)
	return u
}

func (p *AccountMgr) BindAID(aid uint64, u *Account) {
	if old := p.GetByAID(aid); old != nil && old != u {
		xlog.GLog.Infof("duplicate aid login, disconnect old account")
		old.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
	}
	p.byAID.Add(aid, u)
}

// Remove 摘除 remote 对应用户，同步执行 Cleanup，并移除 aid 索引。
func (p *AccountMgr) Remove(remote xnetcommon.IRemote) (*Account, error) {
	u, ok := p.byRemote.Find(remote)
	if !ok {
		return nil, nil
	}
	p.byRemote.Del(remote)

	if current := p.GetByAID(u.aid); current == u {
		p.byAID.Del(u.aid)
	}

	err := u.PostSyncCleanup(remote.GetDisconnectReason())

	u.remote.Stop()
	return u, err
}
