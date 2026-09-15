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
	res := &pb.CharacterSettingSetRes{CharacterUuid: req.GetCharacterUuid()}
	switch action := req.GetAction().(type) {
	case *pb.CharacterSettingSetReq_TeamEnabled:
		// 非队长的组员不允许修改是否允许组队.
		key := sceneCharacterKey{aid: p.aid, characterUUID: character.record.GetBase().GetUuid()}
		member, leader := GCharacterTeamMgr.membership(key)
		if member && !leader && action.TeamEnabled != character.teamEnabled {
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
		character.teamEnabled = action.TeamEnabled
		res.Action = &pb.CharacterSettingSetRes_TeamEnabled{TeamEnabled: character.teamEnabled}
	case *pb.CharacterSettingSetReq_DuelEnabled:
		character.duelEnabled = action.DuelEnabled
		res.Action = &pb.CharacterSettingSetRes_DuelEnabled{DuelEnabled: character.duelEnabled}
	default:
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.refreshCharacterPresence(character)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.Success.Code(), res)
}
