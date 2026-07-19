package main

import (
	"fmt"
	"server/common"
	"server/common/gameconfig"
	commonpet "server/common/pet"
	pb "server/proto/pb"
	"strings"
	"time"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	msgID := pb.MsgID(pkt.GetMessageId())
	switch msgID {
	case pb.MsgID_AccountRecordReq_CMD:
		p.sendClientRes(gateway, uint32(pb.MsgID_AccountRecordRes_CMD), xerror.Success.Code(),
			&pb.AccountRecordRes{
				AccountRecord: p.accountRecord,
			},
		)
		return
	case pb.MsgID_CharacterCreateReq_CMD:
		p.onCharacterCreateReq(gateway, pkt)
		return
	case pb.MsgID_CharacterOnlineReq_CMD:
		p.onCharacterOnlineReq(gateway, pkt)
		return
	case pb.MsgID_CharacterOfflineReq_CMD:
		p.onCharacterOfflineReq(gateway, pkt)
		return
	case pb.MsgID_SceneEnterReq_CMD:
		p.onSceneEnterReq(gateway, pkt)
		return
	case pb.MsgID_CombatAutoEncounterSetReq_CMD:
		p.onAutoEncounterSetReq(gateway, pkt)
		return
	case pb.MsgID_CombatRoundActionReq_CMD:
		p.onCombatRoundActionReq(gateway, pkt)
		return
	case pb.MsgID_CombatFlowCompleteReq_CMD:
		p.onCombatFlowCompleteReq(gateway, pkt)
		return
	case pb.MsgID_RobotPingReq_CMD:
		p.onRobotPingReq(gateway, pkt)
		return
	default:
		xlog.GLog.Warnf("unknown client packet aid:%d messageID:%d", p.aid, pkt.GetMessageId())
		return
	}
}

func (p *Account) sendClientRes(gateway *Gateway, messageID uint32, resultID uint32, message proto.Message) {
	body, err := proto.Marshal(message)
	if err != nil {
		xlog.GLog.Errorf("marshal client response failed aid:%d messageID:%d err:%v", p.aid, messageID, err)
		return
	}
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: 0,
				ResultId:  resultID,
				Key:       p.aid,
				Body:      body,
			},
		},
	})
}

func (p *Account) sendClientErr(gateway *Gateway, messageID uint32, resultID uint32) {
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: 0,
				ResultId:  resultID,
				Key:       p.aid,
				Body:      nil,
			},
		},
	})
}

func (p *Account) onRobotPingReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.RobotPingReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_RobotPingRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_RobotPingRes_CMD), xerror.Success.Code(),
		&pb.RobotPingRes{
			Seq:               req.GetSeq(),
			ClientTimestampMs: req.GetClientTimestampMs(),
			ServerTimestampMs: time.Now().UnixMilli(),
			Payload:           req.GetPayload(),
		},
	)
}

func (p *Account) onCharacterCreateReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterCreateReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// characterSlotIndex
	characterSlotIndex := req.GetCharacterSlotIndex()
	if characterSlotIndex >= uint32(pb.AccountRecordLimit_AccountRecordLimit_MaxCharacterSlotCount) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	slotIndex := int(characterSlotIndex)
	if slotIndex < len(p.accountRecord.CharacterRecordList) {
		character := p.accountRecord.CharacterRecordList[slotIndex]
		if character != nil && character.GetUuid() != 0 {
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.AlreadyExists.Code())
			return
		}
	}

	// character id
	characterCfg := gameconfig.GGameConfig.Character.Get(req.GetCharacterId())
	if characterCfg == nil || !*characterCfg.IsRole {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character nick
	resolvedCharacterNick := strings.TrimSpace(req.GetCharacterNick())
	if !common.IsValidCharacterNick(resolvedCharacterNick) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character elemental
	if !common.IsValidElementalAllocation(req.GetCharacterElemental()) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character attribute
	if req.GetCharacterAttribute().GetVitality()+req.GetCharacterAttribute().GetStrength()+req.GetCharacterAttribute().GetToughness()+req.GetCharacterAttribute().GetDexterity() != uint32(pb.CharacterAttributeLimit_CharacterAttributeLimit_TotalPoint) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	characterUUID := nextAccountRecordUUID(p.accountRecord)
	characterRecord := &pb.CharacterRecord{
		Uuid:              characterUUID,
		Nick:              resolvedCharacterNick,
		AssetId:           uint64(req.GetCharacterId()),
		Exp:               0,
		Earth:             req.GetCharacterElemental().GetEarth(),
		Water:             req.GetCharacterElemental().GetWater(),
		Fire:              req.GetCharacterElemental().GetFire(),
		Wind:              req.GetCharacterElemental().GetWind(),
		AvailablePoint:    0,
		Vitality:          req.CharacterAttribute.GetVitality(),
		Strength:          req.CharacterAttribute.GetStrength(),
		Toughness:         req.CharacterAttribute.GetToughness(),
		Dexterity:         req.CharacterAttribute.GetDexterity(),
		CreateTimestampMs: time.Now().UnixMilli(),
		SceneId:           2000001,
		LuckState:         &pb.CharacterLuckState{},
		AssetIdRecordMap:  make(map[uint32]uint64),
		RecordMap:         make(map[uint64]*pb.RecordPrimary),
		PetRecordList:     make([]*pb.PetRecord, 0, int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount)),
	}

	// pet
	var defaultPetRecords = []struct {
		assetID     uint32
		level       uint32
		carryStatus pb.PetCarryStatus
	}{
		{assetID: 4000277, level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Battle},
		{assetID: 4000278, level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000279, level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000280, level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000360, level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
	}
	if len(defaultPetRecords) > int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		xlog.GLog.Fatalf("default pet count %d exceeds maximum %d", len(defaultPetRecords), pb.PetRecordLimit_PetRecordLimit_MaxCarryCount)
		panic(fmt.Sprintf("default pet count %d exceeds maximum %d", len(defaultPetRecords), pb.PetRecordLimit_PetRecordLimit_MaxCarryCount))
	}
	for _, pet := range defaultPetRecords {
		newPet := gameconfig.GGameConfig.Pet.Get(pet.assetID)
		petUUID := nextAccountRecordUUID(p.accountRecord)
		petRecord := commonpet.NewRecord(newPet, petUUID, pet.level, pb.PetGrade_PetGrade_Mythic)
		petRecord.CarryStatus = pet.carryStatus
		characterRecord.PetRecordList = append(characterRecord.PetRecordList, petRecord)
	}

	p.accountRecord.CharacterRecordList[slotIndex] = characterRecord

	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record failed aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}
	p.characterManager.characters[characterUUID] = &character{
		account: p,
		record:  characterRecord,
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.Success.Code(), &pb.CharacterCreateRes{
		AccountRecord: p.accountRecord,
	})
}

func (p *Account) onCharacterOnlineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOnlineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.NotFound.Code())
		return
	}
	if character.online {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.AlreadyExists.Code())
		return
	}
	nowMs := time.Now().UnixMilli()
	backup, err := prepareCharacterOnlineRecord(character.record, nowMs, randomCharacterLuckRoll)
	if err != nil {
		xlog.GLog.Errorf("prepare character online record failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		backup.restore(character.record)
		xlog.GLog.Errorf("set account record after character online failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	character.clearRuntime()
	character.online = true
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Success.Code(), &pb.CharacterOnlineRes{
		CharacterUuid: characterUUID,
		LuckState: &pb.CharacterLuckState{
			BaseLuck:               character.record.GetLuckState().GetBaseLuck(),
			LastRefreshTimestampMs: character.record.GetLuckState().GetLastRefreshTimestampMs(),
		},
	})
}

func (p *Account) onCharacterOfflineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOfflineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	character.record.LastLogoutTimestampMs = time.Now().UnixMilli()
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after character offline failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.Internal.Code())
		return
	}
	character.clearRuntime()
	character.online = false
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.Success.Code(), &pb.CharacterOfflineRes{
		CharacterUuid: characterUUID,
	})
}

func (p *Account) onSceneEnterReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.SceneEnterReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil { // 角色-不存在
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online { // 角色-不在线
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if character.combatRoom != nil { // 战斗中
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	sceneEntry := gameconfig.GGameConfig.Scene.Get(req.GetSceneId())
	if sceneEntry == nil { // 场景-不存在
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character.record.SceneId = character.record.GetSceneId()
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after scene enter failed aid:%d character:%d scene:%d err:%v", p.aid, characterUUID, req.GetSceneId(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.Internal.Code())
		return
	}
	if character.autoEncounterEnabled {
		character.autoEncounterEnabled = false
		character.clearAutoEncounterTimer()
	}
	// SceneEnter 成功后始终推送 session_id=0 的最终状态. 即使遇敌原本已关闭,
	// 客户端也需要这条权威重置信号同步清除该角色的本地自动战斗开关.
	character.notifyAutoEncounterState(gateway)
	p.sendClientRes(gateway, uint32(pb.MsgID_SceneEnterRes_CMD), xerror.Success.Code(), &pb.SceneEnterRes{
		CharacterUuid:     characterUUID,
		SceneId:           req.GetSceneId(),
		ServerTimestampMs: time.Now().UnixMilli(),
	})
}
