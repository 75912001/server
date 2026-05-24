package main

import (
	"context"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xpacket "github.com/75912001/xlib/packet"
)

type UserShard struct {
	id    int
	actor *xactor.Actor[int]
}

func newUserShard(id int) *UserShard {
	s := &UserShard{id: id}
	s.actor = xactor.NewActor[int](id, nil, s.behavior)
	s.actor.Start()
	return s
}

func (s *UserShard) PostFrame(frame *pb.OnlineTunnelFrame) {
	s.actor.SendMsg(xactor.NewMsg(context.Background(), CmdUserOnlineFrame, frame))
}

func (s *UserShard) PostSyncVerified(user *User, uid uint64, online *Online) error {
	_, err := s.actor.SendMsgSync(xactor.NewMsg(context.Background(), CmdUserVerified, &userVerifiedEvent{
		user:   user,
		uid:    uid,
		online: online,
	}))
	return err
}

func (s *UserShard) PostHeartbeat(user *User, header *xpacket.Header, body []byte) {
	s.actor.SendMsg(xactor.NewMsg(context.Background(), CmdUserHeartbeat, &userHeartbeatEvent{
		user:   user,
		header: header,
		body:   body,
	}))
}

func (s *UserShard) PostCleanup(user *User) error {
	s.actor.SendMsg(xactor.NewMsg(context.Background(), CmdUserCleanup, user))
	return nil
}
