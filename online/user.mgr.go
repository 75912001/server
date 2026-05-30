package main

import (
	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
)

var GUserMgr = &UserMgr{
	users: xmap.NewMapMutexMgr[uint64, *User](),
}

type UserMgr struct {
	users *xmap.MapMutexMgr[uint64, *User]
}

func (p *UserMgr) Login(req *pb.OnlineUserOnlineReq) (*pb.OnlineUserOnlineRes, error) {
	uid := req.GetUid()
	user, ok := p.users.Find(uid)
	if !ok {
		user = newUser(uid)
		p.users.Add(uid, user)
	}
	return user.PostLogin(req)
}

func (p *UserMgr) Remove(uid uint64, user *User) {
	current, ok := p.users.Find(uid)
	if ok && current == user {
		p.users.Del(uid)
	}
}
