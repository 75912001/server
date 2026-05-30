package main

import (
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
)

type User struct {
	uid        uint64
	gatewayID  string
	clientIP   string
	userRecord *pb.UserRecord
	actor      *xactor.Actor[uint64]
}

func newUser(uid uint64) *User {
	u := &User{uid: uid}
	u.actor = xactor.NewActor[uint64](uid, nil, u.behavior)
	u.actor.Start()
	return u
}
