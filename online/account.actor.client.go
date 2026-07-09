package main

import (
	"fmt"
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
	msgID := pb.MsgIDAccount(pkt.GetMessageId())
	switch msgID {
	default:
		if !p.accountRecordReady() {
			xlog.GLog.Warnf("unknown client packet aid:%d messageID:%d", p.aid, pkt.GetMessageId())
			return
		}
	}

	switch msgID {
	case pb.MsgIDAccount_AccountRecordReq_CMD:
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_AccountRecordRes_CMD), xerror.Success.Code(),
			&pb.AccountRecordRes{
				AccountRecord: p.accountRecord,
			},
		)
		return
	case pb.MsgIDAccount_CharacterCreateReq_CMD:
		p.onCharacterCreateReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CharacterOnlineReq_CMD:
		p.onCharacterOnlineReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CharacterOfflineReq_CMD:
		p.onCharacterOfflineReq(gateway, pkt)
		return
	case pb.MsgIDAccount_SceneEnterReq_CMD:
		p.onSceneEnterReq(gateway, pkt)
		return
	case pb.MsgIDAccount_AutoEncounterSetReq_CMD:
		p.onAutoEncounterSetReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CombatRoundActionReq_CMD:
		p.onCombatRoundActionReq(gateway, pkt)
		return
	case pb.MsgIDAccount_RobotPingReq_CMD:
		p.onRobotPingReq(gateway, pkt)
		return
	default:
		xlog.GLog.Warnf("unknown client packet aid:%d messageID:%d", p.aid, pkt.GetMessageId())
		return
	}
}

func (p *Account) sendClientRes(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32, message proto.Message) {
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
				SessionId: pkt.GetSessionId(),
				ResultId:  resultID,
				Key:       p.aid,
				Body:      body,
			},
		},
	})
}

func (p *Account) sendClientErr(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32) {
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: pkt.GetSessionId(),
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
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_RobotPingRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_RobotPingRes_CMD), xerror.Success.Code(),
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
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// characterSlotIndex
	characterSlotIndex := req.GetCharacterSlotIndex()
	if characterSlotIndex >= maxCharacterSlotCount {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	slotIndex := int(characterSlotIndex)
	for len(p.accountRecord.CharacterRecordList) <= slotIndex {
		p.accountRecord.CharacterRecordList = append(p.accountRecord.CharacterRecordList, &pb.CharacterRecord{})
	}
	if character := p.accountRecord.CharacterRecordList[slotIndex]; character != nil && character.GetUuid() != 0 {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.AlreadyExists.Code())
		return
	}

	// character id
	characterCfg := gameconfig.GGameConfig.Character.Get(req.GetCharacterId())
	if characterCfg == nil || !*characterCfg.IsRole {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character nick
	resolvedCharacterNick := strings.TrimSpace(req.GetCharacterNick())
	if !isValidCharacterNick(resolvedCharacterNick) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character elemental
	if !isValidElementalAllocation(req.GetCharacterElemental()) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	// character attribute
	if !isValidCharacterAttributeAllocation(req.GetCharacterAttribute()) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	if p.accountRecord.PetWarehouseRecordMap == nil { // todo menglc 创建角色账号的时候应该创建
		p.accountRecord.PetWarehouseRecordMap = make(map[uint64]*pb.PetRecord)
	}

	characterUUID := nextAccountRecordUUID(p.accountRecord)
	character := &pb.CharacterRecord{
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
		SceneId:           defaultCharacterSceneID,
		AssetIdRecordMap:  make(map[uint32]uint64),
		RecordMap:         make(map[uint64]*pb.RecordPrimary),
		PetRecordList:     make([]*pb.PetRecord, 0, maxPetCarryCount),
	}

	// pet
	var defaultPetRecords = []struct {
		assetID     uint32
		nick        string
		level       uint32
		carryStatus pb.PetCarryStatus
	}{
		{assetID: 4000101, nick: "利则诺顿", level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Battle},
		{assetID: 4000102, nick: "扬奇洛斯", level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000103, nick: "邦浦洛斯", level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000104, nick: "邦奇诺", level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
		{assetID: 4000105, nick: "布鲁顿", level: 1, carryStatus: pb.PetCarryStatus_PetCarryStatus_Wait},
	}
	if len(defaultPetRecords) > maxPetCarryCount {
		xlog.GLog.Fatalf("default pet count %d exceeds maximum %d", len(defaultPetRecords), maxPetCarryCount)
		panic(fmt.Sprintf("default pet count %d exceeds maximum %d", len(defaultPetRecords), maxPetCarryCount))
	}
	for _, pet := range defaultPetRecords {
		newPet := gameconfig.GGameConfig.Pet.Get(pet.assetID)
		petUUID := nextAccountRecordUUID(p.accountRecord)
		petRecord := commonpet.NewRecord(newPet, petUUID, pet.level, pb.PetGrade_PetGrade_Mythic)
		character.PetRecordList = append(character.PetRecordList, petRecord)
	}

	p.accountRecord.CharacterRecordList[slotIndex] = character

	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record failed aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Success.Code(), &pb.CharacterCreateRes{
		AccountRecord: p.accountRecord,
	})
}

func (p *Account) onCharacterOnlineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOnlineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.findCharacterRecord(characterUUID)
	if character == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.NotFound.Code())
		return
	}
	if p.isCharacterOnline(characterUUID) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.AlreadyExists.Code())
		return
	}
	character.LastLoginTimestampMs = time.Now().UnixMilli()
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after character online failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	p.onlineCharacterUUIDSet[characterUUID] = struct{}{}
	p.activeCharacterUUID = characterUUID
	p.autoEncounterEnabled = false
	p.clearAutoEncounterTimer()
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.Success.Code(), &pb.CharacterOnlineRes{})
}

func (p *Account) onCharacterOfflineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOfflineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.findCharacterRecord(characterUUID)
	if character == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.NotFound.Code())
		return
	}
	if !p.isCharacterOnline(characterUUID) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	character.LastLogoutTimestampMs = time.Now().UnixMilli()
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after character offline failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.Internal.Code())
		return
	}
	delete(p.onlineCharacterUUIDSet, characterUUID)
	if p.activeCharacterUUID == characterUUID {
		p.activeCharacterUUID = 0
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.Success.Code(), &pb.CharacterOfflineRes{})
}

func (p *Account) onSceneEnterReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.SceneEnterReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	character := p.findCharacterRecord(characterUUID)
	if character == nil { // 角色-不存在
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.NotFound.Code())
		return
	}
	if !p.isCharacterOnline(characterUUID) { // 角色-不在线
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if p.combatState != nil { // 战斗中
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	sceneEntry := gameconfig.GGameConfig.Scene.Get(req.GetSceneId())
	if sceneEntry == nil { // 场景-不存在
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character.SceneId = req.GetSceneId()
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after scene enter failed aid:%d character:%d scene:%d err:%v", p.aid, characterUUID, req.GetSceneId(), err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.Internal.Code())
		return
	}
	p.activeCharacterUUID = characterUUID
	p.autoEncounterEnabled = false
	p.clearAutoEncounterTimer()
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.Success.Code(), &pb.SceneEnterRes{
		CharacterUuid:     characterUUID,
		SceneId:           req.GetSceneId(),
		ServerTimestampMs: time.Now().UnixMilli(),
	})
}
