package main

import (
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) sendScenePresencePacket(target sceneCharacterPresence, messageID uint32, resultID uint32, message proto.Message) {
	gateway := GGatewayMgr.Get(target.gatewayKey)
	if gateway == nil {
		return
	}
	body, err := proto.Marshal(message)
	if err != nil {
		xlog.GLog.Errorf("marshal scene presence packet failed aid:%d character:%d message:%d err:%v", target.key.aid, target.key.characterUUID, messageID, err)
		return
	}
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: target.key.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				ResultId:  resultID,
				Key:       target.key.aid,
				Body:      body,
			},
		},
	})
}
