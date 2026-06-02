package main

import (
	"server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *User) onClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	if gateway == nil || pkt == nil {
		return
	}
	msgID := pb.MsgIDUser(pkt.GetMessageId())
	switch msgID {
	case pb.MsgIDUser_UserRecordReq_CMD:
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_UserRecordRes_CMD), xerror.Success.Code(), &pb.UserRecordRes{
			UserRecord: p.userRecord,
		})
	case pb.MsgIDUser_UserCreateReq_CMD:
		p.onUserCreateReq(gateway, pkt)
	default:
		if p.userRecord == nil || p.userRecord.GetUid() == 0 {
			p.sendClientRes(gateway, pkt, uint32(msgID), common.ECOnlineUserNotCreated.Code(), nil)
		}
		// xlog.GLog.Warnf("unknown client packet uid:%d messageID:%d", p.uid, pkt.GetMessageId())
	}
	switch msgID {

	}

}

func (p *User) onUserCreateReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.UserCreateReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_UserCreateRes_CMD), xerror.InvalidArgument.Code(), &pb.UserCreateRes{})
		return
	}
	if p.userRecord != nil && p.userRecord.GetUid() != 0 {
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_UserCreateRes_CMD), xerror.AlreadyExists.Code(), &pb.UserCreateRes{
			UserRecord: p.userRecord,
		})
		return
	}
	userRecord := &pb.UserRecord{Uid: p.uid}
	if err := unaryCacheSetUserRecord(p.uid, userRecord); err != nil {
		xlog.GLog.Errorf("set user record failed uid:%d err:%v", p.uid, err)
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_UserCreateRes_CMD), xerror.Internal.Code(), &pb.UserCreateRes{})
		return
	}
	p.userRecord = userRecord
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_UserCreateRes_CMD), xerror.Success.Code(), &pb.UserCreateRes{
		UserRecord: p.userRecord,
	})
}

func (p *User) sendClientRes(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32, message proto.Message) {
	body, err := proto.Marshal(message)
	if err != nil {
		xlog.GLog.Errorf("marshal client response failed uid:%d messageID:%d err:%v", p.uid, messageID, err)
		return
	}
	gateway.Send(&pb.OnlineTunnelFrame{
		Uid: p.uid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: pkt.GetSessionId(),
				ResultId:  resultID,
				Key:       p.uid,
				Body:      body,
			},
		},
	})
}
