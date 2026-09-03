package main

import (
	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onNPCInteractionReq(gateway *Gateway, packet *pb.OnlineClientPacket) {
	var request pb.NpcInteractionReq
	if err := proto.Unmarshal(packet.GetBody(), &request); err != nil ||
		request.GetCharacterUuid() == 0 || request.GetNpcEntityId() == 0 || request.GetOptionId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	challenge := request.GetBattleChallenge()
	if challenge == nil || challenge.GetOperation() != pb.NpcBattleChallengeOperation_NpcBattleChallengeOperation_Start {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(request.GetCharacterUuid())
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil || character.sceneID == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	key := sceneCharacterKey{aid: p.aid, characterUUID: request.GetCharacterUuid()}
	if _, ok := GScenePresenceMgr.get(character.sceneID, key); !ok {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Scene == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.Internal.Code())
		return
	}
	scene := gameconfig.GGameConfig.Scene.Get(character.sceneID)
	if scene == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	option, ok := scene.NPCFunctionOption(request.GetNpcEntityId(), request.GetOptionId())
	if !ok || option.FunctionID == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.NotFound.Code())
		return
	}
	if option.Enabled == nil || !*option.Enabled ||
		*option.FunctionID != pb.NpcFunctionID_NpcFunctionID_BattleChallenge {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	enemyGroupID, ok := option.BattleChallengeEnemyGroupID()
	if !ok {
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.Internal.Code())
		return
	}
	if err := character.startCombatPVE(gateway, enemyGroupID); err != nil {
		xlog.GLog.Warnf(
			"npc battle challenge failed aid:%d character:%d scene:%d npc:%d option:%d enemyGroup:%d err:%v",
			p.aid, request.GetCharacterUuid(), character.sceneID, request.GetNpcEntityId(), request.GetOptionId(), enemyGroupID, err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_NpcInteractionRes_CMD), xerror.Success.Code(), &pb.NpcInteractionRes{
		CharacterUuid: request.GetCharacterUuid(),
		NpcEntityId:   request.GetNpcEntityId(),
		OptionId:      request.GetOptionId(),
		FunctionId:    *option.FunctionID,
	})
}
