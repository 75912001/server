package main

import (
	"server/common"
	pb "server/proto/pb"
	"time"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	msgID := pb.MsgIDUser(pkt.GetMessageId())
	switch msgID {
	case pb.MsgIDUser_AccountRecordReq_CMD:
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_AccountRecordRes_CMD), xerror.Success.Code(),
			&pb.AccountRecordRes{
				AccountRecord: p.accountRecord,
			},
		)
		return
	case pb.MsgIDUser_AccountCreateReq_CMD:
		p.onAccountCreateReq(gateway, pkt)
		return
	default:
		if p.accountRecord == nil || p.accountRecord.GetAccountRecordCreateTimestampMs() == 0 {
			p.sendClientErr(gateway, pkt, uint32(msgID), common.ECOnlineUserNotCreated.Code())
			return
		}
		// xlog.GLog.Warnf("unknown client packet uid:%d messageID:%d", p.uid, pkt.GetMessageId())
	}
	switch msgID {
	case pb.MsgIDUser_RobotPingReq_CMD:
		p.onRobotPingReq(gateway, pkt)
		return
	default:
		xlog.GLog.Warnf("unknown client packet uid:%d messageID:%d", p.uid, pkt.GetMessageId())
		return
	}
}

func (p *Account) onRobotPingReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.RobotPingReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_RobotPingRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_RobotPingRes_CMD), xerror.Success.Code(),
		&pb.RobotPingRes{
			Seq:               req.GetSeq(),
			ClientTimestampMs: req.GetClientTimestampMs(),
			ServerTimestampMs: time.Now().UnixMilli(),
			Payload:           req.GetPayload(),
		},
	)
}

func (p *Account) onAccountCreateReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.AccountCreateReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	if p.accountRecord == nil {
		xlog.GLog.Errorf("account record is nil uid:%d", p.uid)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.Internal.Code())
		return
	}

	if p.accountRecord.GetAccountRecordCreateTimestampMs() != 0 {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.AlreadyExists.Code())
		return
	}
	now := time.Now().UnixMilli()
	if err := initializeDefaultAccountRecord(p.accountRecord, now); err != nil {
		xlog.GLog.Errorf("initialize account record failed uid:%d err:%v", p.uid, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.Internal.Code())
		return
	}
	if err := unaryCacheSetAccountRecord(p.uid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record failed uid:%d err:%v", p.uid, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDUser_AccountCreateRes_CMD), xerror.Success.Code(), &pb.AccountCreateRes{
		AccountRecord: p.accountRecord,
	})
}

func (p *Account) sendClientRes(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32, message proto.Message) {
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

func (p *Account) sendClientErr(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32) {
	gateway.Send(&pb.OnlineTunnelFrame{
		Uid: p.uid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: pkt.GetSessionId(),
				ResultId:  resultID,
				Key:       p.uid,
				Body:      nil,
			},
		},
	})
}
