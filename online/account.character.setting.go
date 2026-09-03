package main

import (
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onCharacterSettingSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterSettingSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	key := sceneCharacterKey{aid: p.aid, characterUUID: character.record.GetBase().GetUuid()}
	member, leader := GCharacterTeamMgr.membership(key)
	if member && !leader && req.GetTeamEnabled() != character.teamEnabled {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	character.teamEnabled = req.GetTeamEnabled()
	character.duelEnabled = req.GetDuelEnabled()
	p.refreshCharacterPresence(character)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.Success.Code(), &pb.CharacterSettingSetRes{
		CharacterUuid: req.GetCharacterUuid(),
		TeamEnabled:   character.teamEnabled,
		DuelEnabled:   character.duelEnabled,
	})
}
