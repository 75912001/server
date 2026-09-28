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
		effectiveAttributeList, err := characterEffectiveAttributeList(p.accountRecord)
		if err != nil {
			xlog.GLog.Errorf("calculate account effective attributes failed aid:%d err:%v", p.aid, err)
			p.sendClientErr(gateway, uint32(pb.MsgID_AccountRecordRes_CMD), xerror.Internal.Code())
			return
		}
		p.sendClientRes(gateway, uint32(pb.MsgID_AccountRecordRes_CMD), xerror.Success.Code(),
			&pb.AccountRecordRes{
				AccountRecord:                   p.accountRecord,
				CharacterEffectiveAttributeList: effectiveAttributeList,
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
	case pb.MsgID_CharacterMapEnterReq_CMD:
		p.onCharacterMapEnterReq(gateway, pkt)
		return
	case pb.MsgID_MovePathReq_CMD:
		p.onMovePathReq(gateway, pkt)
		return
	case pb.MsgID_NpcInteractionReq_CMD:
		p.onNPCInteractionReq(gateway, pkt)
		return
	case pb.MsgID_CharacterSettingSetReq_CMD:
		p.onCharacterSettingSetReq(gateway, pkt)
		return
	case pb.MsgID_CharacterTeamOperationReq_CMD:
		p.onCharacterTeamOperationReq(gateway, pkt)
		return
	case pb.MsgID_CharacterAttributeAddReq_CMD:
		p.onCharacterAttributeAddReq(gateway, pkt)
		return
	case pb.MsgID_CharacterAttributeResetReq_CMD:
		p.onCharacterAttributeResetReq(gateway, pkt)
		return
	case pb.MsgID_CharacterEquipmentReplaceReq_CMD:
		p.onCharacterEquipmentReplaceReq(gateway, pkt)
		return
	case pb.MsgID_PetCarryStatusSetReq_CMD:
		p.onPetCarryStatusSetReq(gateway, pkt)
		return
	case pb.MsgID_PetWarehouseDepositReq_CMD:
		p.onPetWarehouseDepositReq(gateway, pkt)
		return
	case pb.MsgID_PetWarehouseWithdrawReq_CMD:
		p.onPetWarehouseWithdrawReq(gateway, pkt)
		return
	case pb.MsgID_PetNickSetReq_CMD:
		p.onPetNickSetReq(gateway, pkt)
		return
	case pb.MsgID_PetSkillSetReq_CMD:
		p.onPetSkillSetReq(gateway, pkt)
		return
	case pb.MsgID_ItemWarehouseDepositReq_CMD:
		p.onItemWarehouseDepositReq(gateway, pkt)
		return
	case pb.MsgID_ItemWarehouseWithdrawReq_CMD:
		p.onItemWarehouseWithdrawReq(gateway, pkt)
		return
	case pb.MsgID_ItemUseReq_CMD:
		p.onItemUseReq(gateway, pkt)
		return
	case pb.MsgID_ShopPurchaseReq_CMD:
		p.onShopPurchaseReq(gateway, pkt)
		return
	case pb.MsgID_ItemSynthesisReq_CMD:
		p.onItemSynthesisReq(gateway, pkt)
		return
	case pb.MsgID_EquipmentSkillAttachReq_CMD:
		p.onEquipmentSkillAttachReq(gateway, pkt)
		return
	case pb.MsgID_EquipmentElementAttachReq_CMD:
		p.onEquipmentElementAttachReq(gateway, pkt)
		return
	case pb.MsgID_GMCommandReq_CMD:
		p.onGMCommandReq(gateway, pkt)
		return
	case pb.MsgID_CharacterMailboxGetReq_CMD:
		p.onCharacterMailboxGetReq(gateway, pkt)
		return
	case pb.MsgID_CharacterMailReadReq_CMD:
		p.onCharacterMailReadReq(gateway, pkt)
		return
	case pb.MsgID_CharacterMailDeleteReq_CMD:
		p.onCharacterMailDeleteReq(gateway, pkt)
		return
	case pb.MsgID_TaskAcceptReq_CMD:
		p.onTaskAcceptReq(gateway, pkt)
		return
	case pb.MsgID_TaskSubmitReq_CMD:
		p.onTaskSubmitReq(gateway, pkt)
		return
	case pb.MsgID_TaskStepRewardClaimReq_CMD:
		p.onTaskStepRewardClaimReq(gateway, pkt)
		return
	case pb.MsgID_TaskBattleChallengeReq_CMD:
		p.onTaskBattleChallengeReq(gateway, pkt)
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
	case pb.MsgID_AccountRobotPingReq_CMD:
		p.onAccountRobotPingReq(gateway, pkt)
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

func characterBaseRecord(record *pb.CharacterRecord) *pb.CharacterBaseRecord {
	if record == nil || record.GetBase() == nil {
		return nil
	}
	return proto.Clone(record.GetBase()).(*pb.CharacterBaseRecord)
}

// sendCharacterNotify 是角色数据变化通知的唯一出口: 所有域变化统一走
// CharacterNotify(0x001011) 的 oneof 分支下发, 一次操作多域同变时按域连发多条.
// 调用方必须构造非空 Change 且设置有效 character_uuid.
func (p *Account) sendCharacterNotify(gateway *Gateway, notify *pb.CharacterNotify) {
	if gateway == nil || notify == nil || notify.GetCharacterUuid() == 0 || notify.GetChange() == nil {
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterNotify_CMD), xerror.Success.Code(), notify)
}

func (p *Account) sendCharacterBaseChangedNotify(gateway *Gateway, record *pb.CharacterRecord) {
	base := characterBaseRecord(record)
	if base == nil {
		return
	}
	effective, err := characterEffectiveAttribute(record)
	if err != nil {
		xlog.GLog.Errorf("calculate changed character effective attribute failed aid:%d character:%d err:%v", p.aid, base.GetUuid(), err)
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: base.GetUuid(),
		Change: &pb.CharacterNotify_BaseChanged{
			BaseChanged: &pb.CharacterBaseChanged{
				CharacterBaseRecord: base,
				EffectiveAttribute:  effective,
			},
		},
	})
	if character := p.characterManager.find(base.GetUuid()); character != nil {
		p.refreshCharacterPresence(character)
	}
}

// sendCharacterPetChangedNotify 下发新增或变化宠物的完整记录(按 uuid 合并).
func (p *Account) sendCharacterPetChangedNotify(gateway *Gateway, characterUUID uint64, petRecordList []*pb.PetRecord) {
	changedPetRecordList := make([]*pb.PetRecord, 0, len(petRecordList))
	for _, petRecord := range petRecordList {
		if petRecord != nil {
			changedPetRecordList = append(changedPetRecordList, proto.Clone(petRecord).(*pb.PetRecord))
		}
	}
	if characterUUID == 0 || len(changedPetRecordList) == 0 {
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_PetChanged{
			PetChanged: &pb.CharacterPetChanged{PetRecordList: changedPetRecordList},
		},
	})
}

// sendCharacterPetRemovedNotify 下发被移出随身列表的宠物 UUID(消耗/离队等).
func (p *Account) sendCharacterPetRemovedNotify(gateway *Gateway, characterUUID uint64, removedPetUUIDs []uint64) {
	if characterUUID == 0 || len(removedPetUUIDs) == 0 {
		return
	}
	removedUUIDList := make([]uint64, 0, len(removedPetUUIDs))
	for _, petUUID := range removedPetUUIDs {
		if petUUID != 0 {
			removedUUIDList = append(removedUUIDList, petUUID)
		}
	}
	if len(removedUUIDList) == 0 {
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_PetChanged{
			PetChanged: &pb.CharacterPetChanged{RemovedPetUuidList: removedUUIDList},
		},
	})
}

func (p *Account) sendCharacterItemChangedNotify(gateway *Gateway, characterUUID uint64, itemCountMap map[uint32]uint64) {
	if characterUUID == 0 || len(itemCountMap) == 0 {
		return
	}
	changedItemCountMap := make(map[uint32]uint64, len(itemCountMap))
	for itemID, count := range itemCountMap {
		changedItemCountMap[itemID] = count
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_ItemChanged{
			ItemChanged: &pb.CharacterItemChanged{ItemCountMap: changedItemCountMap},
		},
	})
}

// sendCharacterContainerChangedNotify 下发完整背包容器快照(装备实例增删等结构性变化).
// usedUUID 仅在本次变化新增了装备实例时传入; 无新增实例传 0.
func (p *Account) sendCharacterContainerChangedNotify(gateway *Gateway, characterUUID uint64, itemBag *pb.ItemContainerRecord, usedUUID uint64) {
	if characterUUID == 0 || itemBag == nil {
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_ContainerChanged{
			ContainerChanged: &pb.CharacterContainerChanged{
				ItemBag:  proto.Clone(itemBag).(*pb.ItemContainerRecord),
				UsedUuid: usedUUID,
			},
		},
	})
}

func (p *Account) sendCharacterEquipmentChangedNotify(gateway *Gateway, characterUUID uint64, character *pb.CharacterRecord, effective *pb.CharacterEffectiveAttribute) {
	if characterUUID == 0 || character == nil || character.GetItemBag() == nil || character.GetEquipment() == nil || effective == nil {
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_EquipmentChanged{EquipmentChanged: &pb.CharacterEquipmentChanged{
			ItemBag:            proto.Clone(character.GetItemBag()).(*pb.ItemContainerRecord),
			Equipment:          proto.Clone(character.GetEquipment()).(*pb.CharacterEquipmentRecord),
			EffectiveAttribute: proto.Clone(effective).(*pb.CharacterEffectiveAttribute),
		}},
	})
}

func (p *Account) sendCharacterTaskChangedNotify(gateway *Gateway, characterUUID uint64, taskRecordMap map[uint32]*pb.CharacterTaskRecord) {
	if characterUUID == 0 || len(taskRecordMap) == 0 {
		return
	}
	changedTaskRecordMap := make(map[uint32]*pb.CharacterTaskRecord, len(taskRecordMap))
	for taskID, taskRecord := range taskRecordMap {
		if taskRecord != nil {
			changedTaskRecordMap[taskID] = proto.Clone(taskRecord).(*pb.CharacterTaskRecord)
		}
	}
	if len(changedTaskRecordMap) == 0 {
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterTaskChangedNotify_CMD), xerror.Success.Code(), &pb.CharacterTaskChangedNotify{
		CharacterUuid:        characterUUID,
		ChangedTaskRecordMap: changedTaskRecordMap,
	})
}

func (p *Account) sendCharacterSystemMailNotify(gateway *Gateway, characterUUID uint64, mailRecord *pb.MailRecord) {
	if characterUUID == 0 || mailRecord == nil {
		return
	}
	p.sendCharacterNotify(gateway, &pb.CharacterNotify{
		CharacterUuid: characterUUID,
		Change: &pb.CharacterNotify_SystemMailChanged{
			SystemMailChanged: &pb.CharacterSystemMailChanged{
				MailRecord: proto.Clone(mailRecord).(*pb.MailRecord),
			},
		},
	})
}

// sendCharacterTaskSettlementNotify 拆分原任务库存全量通知: 任务提交/领奖可能同时
// 改变背包容器、货币资产和随身宠物, 按域连发多条同 CMD 的 CharacterNotify,
// 客户端按到达顺序逐条应用后即与权威一致. 仅发送实际变化的域.
func (p *Account) sendCharacterTaskSettlementNotify(gateway *Gateway, plan *characterTaskMutationPlan) {
	if plan == nil || !plan.inventoryChanged || plan.previous == nil || plan.next == nil {
		return
	}
	p.sendCharacterSettlementNotify(gateway, plan.characterUUID, plan.previous, plan.next, plan.previousUsedUUID, plan.nextUsedUUID)
}

// sendCharacterSettlementNotify 按域发送一次角色库存事务的最终权威状态.
// 任务领奖和可开启道具共用该入口, 保证背包、资产、宠物和UUID的通知顺序一致.
func (p *Account) sendCharacterSettlementNotify(
	gateway *Gateway,
	characterUUID uint64,
	previous *pb.CharacterRecord,
	next *pb.CharacterRecord,
	previousUsedUUID uint64,
	nextUsedUUID uint64,
) {
	if previous == nil || next == nil {
		return
	}
	// 1. 背包容器: item_bag 结构变化(含装备实例增减)或账号 UUID 游标推进时发完整快照.
	usedUUID := uint64(0)
	if nextUsedUUID > previousUsedUUID {
		usedUUID = nextUsedUUID
	}
	if !proto.Equal(previous.GetItemBag(), next.GetItemBag()) || usedUUID != 0 {
		p.sendCharacterContainerChangedNotify(gateway, characterUUID, next.GetItemBag(), usedUUID)
	}
	// 2. 货币资产: 只发变化项的最终数量, 0 表示耗尽.
	if changedAsset := diffAssetCountMap(previous.GetAssetCountMap(), next.GetAssetCountMap()); len(changedAsset) > 0 {
		p.sendCharacterItemChangedNotify(gateway, characterUUID, changedAsset)
	}
	// 3. 随身宠物: 新增/变化记录与移除 UUID 分开发送(同一分支两个字段独立应用).
	changedPets, removedPetUUIDs := diffPetRecordList(previous.GetPetRecordList(), next.GetPetRecordList())
	if len(changedPets) > 0 {
		p.sendCharacterPetChangedNotify(gateway, characterUUID, changedPets)
	}
	if len(removedPetUUIDs) > 0 {
		p.sendCharacterPetRemovedNotify(gateway, characterUUID, removedPetUUIDs)
	}
}

// diffAssetCountMap 返回 previous 与 next 间值有差异的资产项最终数量, 0 表示耗尽/移除.
func diffAssetCountMap(previous, next map[uint32]uint64) map[uint32]uint64 {
	changed := make(map[uint32]uint64)
	for assetID, nextCount := range next {
		if previous[assetID] != nextCount {
			changed[assetID] = nextCount
		}
	}
	for assetID := range previous {
		if _, exists := next[assetID]; !exists {
			changed[assetID] = 0
		}
	}
	return changed
}

// diffPetRecordList 返回宠物列表从 previous 到 next 的变化:
// 新增或内容变化的完整记录列表, 以及被移出列表的 UUID.
func diffPetRecordList(previous, next []*pb.PetRecord) ([]*pb.PetRecord, []uint64) {
	nextByUUID := make(map[uint64]*pb.PetRecord, len(next))
	for _, petRecord := range next {
		if petRecord != nil {
			nextByUUID[petRecord.GetUuid()] = petRecord
		}
	}
	changed := make([]*pb.PetRecord, 0)
	for _, petRecord := range next {
		if petRecord == nil {
			continue
		}
		previousRecord := findPetRecordByUUID(previous, petRecord.GetUuid())
		if previousRecord == nil || !proto.Equal(previousRecord, petRecord) {
			changed = append(changed, proto.Clone(petRecord).(*pb.PetRecord))
		}
	}
	removed := make([]uint64, 0)
	for _, petRecord := range previous {
		if petRecord != nil {
			if _, exists := nextByUUID[petRecord.GetUuid()]; !exists {
				removed = append(removed, petRecord.GetUuid())
			}
		}
	}
	return changed, removed
}

func findPetRecordByUUID(petRecordList []*pb.PetRecord, petUUID uint64) *pb.PetRecord {
	for _, petRecord := range petRecordList {
		if petRecord != nil && petRecord.GetUuid() == petUUID {
			return petRecord
		}
	}
	return nil
}

func (p *Account) onAccountRobotPingReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.AccountRobotPingReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_AccountRobotPingRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_AccountRobotPingRes_CMD), xerror.Success.Code(),
		&pb.AccountRobotPingRes{
			Seq:               req.GetSeq(),
			ClientTimestampMs: req.GetClientTimestampMs(),
			ServerTimestampMs: time.Now().UnixMilli(),
			Payload:           req.GetPayload(),
		},
	)
}

// newCharacterRecord 使用已校验的创建参数构造完整角色档案; 默认宠物和账号 UUID 游标由调用方处理.
func newCharacterRecord(characterUUID uint64, resolvedCharacterNick string, req *pb.CharacterCreateReq, createTimestampMs int64) *pb.CharacterRecord {
	return &pb.CharacterRecord{
		Base: &pb.CharacterBaseRecord{
			Uuid:              characterUUID,
			Nick:              resolvedCharacterNick,
			AssetId:           uint64(req.GetCharacterId()),
			Elemental:         req.GetCharacterElemental(),
			DuelPoint:         characterInitialDuelPoint,
			Charm:             characterInitialCharm,
			Attribute:         req.GetCharacterAttribute(),
			CreateTimestampMs: createTimestampMs,
			LuckState:         &pb.CharacterLuckState{},
		},
		ItemBag: &pb.ItemContainerRecord{
			ItemCountMap:       make(map[uint32]uint64),
			EquipmentRecordMap: make(map[uint64]*pb.EquipmentRecord),
		},
		Equipment:     &pb.CharacterEquipmentRecord{},
		PetRecordList: make([]*pb.PetRecord, 0, int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount)),
		TaskRecordMap: make(map[uint32]*pb.CharacterTaskRecord),
	}
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
		if character != nil && character.GetBase().GetUuid() != 0 {
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
	if req.GetCharacterAttribute().GetVitality()+req.GetCharacterAttribute().GetStrength()+req.GetCharacterAttribute().GetToughness()+req.GetCharacterAttribute().GetDexterity() != uint32(pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	previousUsedUUID := p.accountRecord.GetUsedUuid()
	previousCharacterRecord := p.accountRecord.CharacterRecordList[slotIndex]
	characterUUID := nextAccountRecordUUID(p.accountRecord)
	characterRecord := newCharacterRecord(characterUUID, resolvedCharacterNick, &req, time.Now().UnixMilli())

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
		petRecord, err := commonpet.NewRecord(newPet, petUUID, pet.level, pb.PetGrade_PetGrade_Mythic)
		if err != nil {
			// 角色槽位尚未写入, 这里只需还原本轮已经分配的角色和宠物 UUID.
			p.accountRecord.UsedUuid = previousUsedUUID
			xlog.GLog.Errorf("create default pet failed aid:%d pet:%d err:%v", p.aid, pet.assetID, err)
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.Internal.Code())
			return
		}
		petRecord.CarryStatus = pet.carryStatus
		characterRecord.PetRecordList = append(characterRecord.PetRecordList, petRecord)
	}

	p.accountRecord.CharacterRecordList[slotIndex] = characterRecord

	if err := p.deferAccountRecordPersist(); err != nil {
		p.accountRecord.UsedUuid = previousUsedUUID
		p.accountRecord.CharacterRecordList[slotIndex] = previousCharacterRecord
		xlog.GLog.Errorf("set account record failed aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}
	p.characterManager.characters[characterUUID] = &character{
		account: p,
		record:  characterRecord,
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterCreateRes_CMD), xerror.Success.Code(), &pb.CharacterCreateRes{
		CharacterRecord: characterRecord,
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
	effective, err := characterEffectiveAttribute(character.record)
	if err != nil {
		backup.restore(character.record)
		xlog.GLog.Errorf("calculate character online effective attribute failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	if err := p.deferAccountRecordPersist(); err != nil {
		backup.restore(character.record)
		xlog.GLog.Errorf("set account record after character online failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	character.clearRuntime()
	character.online = true
	luck := &pb.CharacterLuckState{
		BaseLuck:               character.record.GetBase().GetLuckState().GetBaseLuck(),
		LastRefreshTimestampMs: character.record.GetBase().GetLuckState().GetLastRefreshTimestampMs(),
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterOnlineRes_CMD), xerror.Success.Code(), &pb.CharacterOnlineRes{
		CharacterUuid:      characterUUID,
		LuckState:          luck,
		EffectiveAttribute: effective,
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
	character.record.Base.LastLogoutTimestampMs = time.Now().UnixMilli()
	if err := p.deferAccountRecordPersist(); err != nil {
		xlog.GLog.Errorf("set account record after character offline failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.Internal.Code())
		return
	}
	sceneID := character.sceneID
	key := sceneCharacterKey{aid: p.aid, characterUUID: characterUUID}
	p.dischargeCharacterTeam(key)
	character.clearRuntime()
	character.online = false
	p.removeCharacterPresence(sceneID, key)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterOfflineRes_CMD), xerror.Success.Code(), &pb.CharacterOfflineRes{
		CharacterUuid: characterUUID,
	})
}
