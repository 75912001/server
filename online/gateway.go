package main

import (
	"context"

	xactor "github.com/75912001/xlib/actor"

	pb "server/proto/pb"
)

// Gateway stores gateway stream state. Downstream frames are serialized by the actor.
type Gateway struct {
	Key string // gateway etcd key

	actor  *xactor.Actor[string]
	stream pb.OnlineService_OnlineStreamTunnelServer
}

func newGateway(key string) *Gateway {
	p := &Gateway{Key: key}
	p.actor = xactor.NewActor[string](key, nil, p.streamBehavior)
	p.actor.Start()
	return p
}

func (p *Gateway) GetID() string { return p.Key }

func (p *Gateway) Stop() error {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	return nil
}
