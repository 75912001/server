package main

import (
	"server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onlineMailboxCharacter(characterUUID uint64) (*character, uint32) {
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		return nil, xerror.NotFound.Code()
	}
	if !character.online {
		return nil, xerror.FailedPrecondition.Code()
	}
	return character, xerror.Success.Code()
}

func (p *Account) onCharacterMailboxGetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterMailboxGetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailboxGetRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if _, resultID := p.onlineMailboxCharacter(req.GetCharacterUuid()); resultID != xerror.Success.Code() {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailboxGetRes_CMD), resultID)
		return
	}
	mailboxRecord, err := unaryCacheGetCharacterMailbox(p.aid, req.GetCharacterUuid())
	if err != nil {
		xlog.GLog.Errorf("get character mailbox failed aid:%d character:%d err:%v", p.aid, req.GetCharacterUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailboxGetRes_CMD), common.GRPCStatusToResultID(err))
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterMailboxGetRes_CMD), xerror.Success.Code(), &pb.CharacterMailboxGetRes{
		CharacterUuid: req.GetCharacterUuid(),
		MailboxRecord: mailboxRecord,
	})
}

func (p *Account) onCharacterMailReadReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterMailReadReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetMailUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailReadRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if _, resultID := p.onlineMailboxCharacter(req.GetCharacterUuid()); resultID != xerror.Success.Code() {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailReadRes_CMD), resultID)
		return
	}
	if err := unaryCacheMarkCharacterMailRead(p.aid, req.GetCharacterUuid(), req.GetMailUuid()); err != nil {
		xlog.GLog.Errorf("mark character mail read failed aid:%d character:%d mail:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetMailUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailReadRes_CMD), common.GRPCStatusToResultID(err))
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterMailReadRes_CMD), xerror.Success.Code(), &pb.CharacterMailReadRes{
		CharacterUuid: req.GetCharacterUuid(),
		MailUuid:      req.GetMailUuid(),
	})
}

func (p *Account) onCharacterMailDeleteReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterMailDeleteReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetMailUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailDeleteRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if _, resultID := p.onlineMailboxCharacter(req.GetCharacterUuid()); resultID != xerror.Success.Code() {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailDeleteRes_CMD), resultID)
		return
	}
	if err := unaryCacheDeleteCharacterMail(p.aid, req.GetCharacterUuid(), req.GetMailUuid()); err != nil {
		xlog.GLog.Errorf("delete character mail failed aid:%d character:%d mail:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetMailUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMailDeleteRes_CMD), common.GRPCStatusToResultID(err))
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterMailDeleteRes_CMD), xerror.Success.Code(), &pb.CharacterMailDeleteRes{
		CharacterUuid: req.GetCharacterUuid(),
		MailUuid:      req.GetMailUuid(),
	})
}
