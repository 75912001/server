package main

import (
	"fmt"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

type equipmentSkillAttachPlan struct {
	characterUUID uint64
	petUUID       uint64
	equipmentUUID uint64
	matched       bool
	skillID       uint32
	materials     []*pb.ItemElement
	characterSlot int
	// materialCosts 在 apply 阶段按消耗后的权威数量填充, 供响应返回.
	materialCosts []*pb.ItemCostResult
}

func (p *Account) onEquipmentSkillAttachReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.EquipmentSkillAttachReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 || req.GetEquipmentUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentSkillAttachRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if err := validateItemSynthesisCharacterState(character); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentSkillAttachRes_CMD), itemSynthesisResultID(err))
		return
	}
	plan, err := prepareEquipmentSkillAttachPlan(p.accountRecord, character.record, req.GetPetUuid(), req.GetEquipmentUuid(), req.GetMaterialList())
	if err != nil {
		xlog.GLog.Warnf("equipment skill attach rejected aid:%d character:%d pet:%d equipment:%d materialKinds:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetPetUuid(), req.GetEquipmentUuid(), len(req.GetMaterialList()), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentSkillAttachRes_CMD), itemSynthesisResultID(err))
		return
	}
	if err := applyEquipmentSkillAttachPlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf("persist equipment skill attach failed aid:%d character:%d pet:%d equipment:%d matched:%t err:%v", p.aid, plan.characterUUID, plan.petUUID, plan.equipmentUUID, plan.matched, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentSkillAttachRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendCharacterContainerChangedNotify(gateway, plan.characterUUID, character.record.GetItemBag(), 0)
	p.sendClientRes(gateway, uint32(pb.MsgID_EquipmentSkillAttachRes_CMD), xerror.Success.Code(), &pb.EquipmentSkillAttachRes{
		CharacterUuid:          plan.characterUUID,
		PetUuid:                plan.petUUID,
		EquipmentUuid:          plan.equipmentUUID,
		Matched:                plan.matched,
		ResultEquipment:        proto.Clone(character.record.GetItemBag().GetEquipmentRecordMap()[plan.equipmentUUID]).(*pb.EquipmentRecord),
		MaterialCostResultList: cloneItemCostResults(plan.materialCosts),
	})
}

// prepareEquipmentSkillAttachPlan 只做校验与变更计算, 不修改账号档案。
// 原地提交要求 apply 阶段不可失败, 因此所有可能失败的检查都在这里完成。
func prepareEquipmentSkillAttachPlan(accountRecord *pb.AccountRecord, characterRecord *pb.CharacterRecord, petUUID, equipmentUUID uint64, materials []*pb.ItemElement) (*equipmentSkillAttachPlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || petUUID == 0 || equipmentUUID == 0 || len(materials) == 0 || len(materials) > gameconfig.TiangongMaximumMaterialTypes {
		return nil, errItemSynthesisInvalidArgument
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Skill == nil || gameconfig.GGameConfig.Tiangong == nil {
		return nil, fmt.Errorf("%w: item, skill or tiangong config is not loaded", errItemSynthesisRecordInvalid)
	}
	if err := validateItemSynthesisPet(characterRecord, petUUID); err != nil {
		return nil, err
	}
	// 与基础加工一致, 必须使用操作前的实际占位数判断空位.
	if itemContainerCount(characterRecord.GetItemBag()) >= int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return nil, fmt.Errorf("%w: item bag has no empty slot", errItemSynthesisFailedPrecondition)
	}
	equipment := characterRecord.GetItemBag().GetEquipmentRecordMap()[equipmentUUID]
	if equipment == nil {
		return nil, fmt.Errorf("%w: equipment %d is not in character bag", errItemSynthesisTargetNotFound, equipmentUUID)
	}
	if err := validateEquipmentRecord(equipment, equipmentUUID); err != nil {
		return nil, fmt.Errorf("%w: equipment %d: %v", errItemSynthesisRecordInvalid, equipmentUUID, err)
	}
	materialCounts, _, _, err := validateItemSynthesisMaterials(characterRecord, materials)
	if err != nil {
		return nil, err
	}

	characterSlot := -1
	for index, candidate := range accountRecord.GetCharacterRecordList() {
		if candidate == characterRecord && candidate.GetBase().GetUuid() == characterRecord.GetBase().GetUuid() {
			characterSlot = index
			break
		}
	}
	if characterSlot < 0 {
		return nil, fmt.Errorf("%w: character record slot not found", errItemSynthesisRecordInvalid)
	}

	enchantment, matched := gameconfig.GGameConfig.Tiangong.MatchEnchantment(equipment.GetAssetId(), materialCounts)
	if matched && equipment.GetAdditionalSkillId() == enchantment.SkillID {
		return nil, fmt.Errorf("%w: equipment %d already has additional skill %d", errItemSynthesisFailedPrecondition, equipmentUUID, enchantment.SkillID)
	}
	// MatchEnchantment 未命中时返回 nil, 只有命中才读取技能 ID.
	skillID := uint32(0)
	if matched {
		skillID = enchantment.SkillID
	}

	return &equipmentSkillAttachPlan{
		characterUUID: characterRecord.GetBase().GetUuid(),
		petUUID:       petUUID,
		equipmentUUID: equipmentUUID,
		matched:       matched,
		skillID:       skillID,
		materials:     materials,
		characterSlot: characterSlot,
	}, nil
}

// applyEquipmentSkillAttachPlan 把计划原地应用到权威账号档案, 再通知落盘。
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知。
func applyEquipmentSkillAttachPlan(plan *equipmentSkillAttachPlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errItemSynthesisInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != character.record {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errItemSynthesisRecordInvalid)
	}
	equipment := character.record.GetItemBag().GetEquipmentRecordMap()[plan.equipmentUUID]
	if equipment == nil {
		return fmt.Errorf("%w: equipment %d is missing before apply", errItemSynthesisRecordInvalid, plan.equipmentUUID)
	}

	itemManager := newCharacterItemManager(character.record)
	for _, material := range plan.materials {
		if err := itemManager.Consume(material.GetAssetId(), material.GetCount()); err != nil {
			return fmt.Errorf("%w: consume material %d: %v", errItemSynthesisRecordInvalid, material.GetAssetId(), err)
		}
	}
	if plan.matched {
		equipment.AdditionalSkillId = plan.skillID
		if err := validateEquipmentRecord(equipment, plan.equipmentUUID); err != nil {
			return fmt.Errorf("%w: attached equipment %d: %v", errItemSynthesisRecordInvalid, plan.equipmentUUID, err)
		}
	}

	plan.materialCosts = plan.materialCosts[:0]
	for _, material := range plan.materials {
		plan.materialCosts = append(plan.materialCosts, &pb.ItemCostResult{
			ItemId:         material.GetAssetId(),
			ConsumedCount:  material.GetCount(),
			RemainingCount: itemManager.Count(material.GetAssetId()),
		})
	}
	return persist()
}
