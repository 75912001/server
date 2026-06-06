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

func (p *UserMgr) GetByUID(uid uint64) *User {
	user, ok := p.users.Find(uid)
	if !ok {
		return nil
	}
	return user
}

func (p *UserMgr) Bind(uid uint64, req *pb.OnlineBindUserReq, userRecord *pb.UserRecord) (*pb.OnlineBindUserRes, error) {
	user, existed := p.users.Find(uid)
	if !existed {
		user = newUser(uid)
		p.users.Add(uid, user)
	}
	res, err := user.PostBind(req, userRecord)
	if err != nil {
		current, ok := p.users.Find(uid)
		if !existed {
			if !ok || current == user {
				p.users.Del(uid)
			}
			user.Stop()
		} else if !ok {
			user.Stop()
		}
		return nil, err
	}
	return res, nil
}
