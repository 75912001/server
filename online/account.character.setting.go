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
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.NotFound.Code())
		return
	}

	character.teamEnabled = req.GetTeamEnabled()
	character.duelEnabled = req.GetDuelEnabled()
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterSettingSetRes_CMD), xerror.Success.Code(), &pb.CharacterSettingSetRes{
		CharacterUuid: req.GetCharacterUuid(),
		TeamEnabled:   character.teamEnabled,
		DuelEnabled:   character.duelEnabled,
	})
}
