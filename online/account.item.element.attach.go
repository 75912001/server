package main

import (
	"fmt"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

const equipmentAttachedElementValue = uint32(20)

type equipmentElementAttachPlan struct {
	characterUUID     uint64
	petUUID           uint64
	equipmentUUID     uint64
	matched           bool
	resultEquipment   *pb.EquipmentRecord
	materialCosts     []*pb.ItemCostResult
	previousUsedUUID  uint64
	characterSlot     int
	previousCharacter *pb.CharacterRecord
	nextCharacter     *pb.CharacterRecord
	nextAccountRecord *pb.AccountRecord
}

func (p *Account) onEquipmentElementAttachReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.EquipmentElementAttachReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 || req.GetEquipmentUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentElementAttachRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if err := validateItemSynthesisCharacterState(character); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentElementAttachRes_CMD), itemSynthesisResultID(err))
		return
	}
	plan, err := prepareEquipmentElementAttachPlan(p.accountRecord, character.record, req.GetPetUuid(), req.GetEquipmentUuid(), req.GetMaterialList())
	if err != nil {
		xlog.GLog.Warnf("equipment element attach rejected aid:%d character:%d pet:%d equipment:%d materialKinds:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetPetUuid(), req.GetEquipmentUuid(), len(req.GetMaterialList()), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentElementAttachRes_CMD), itemSynthesisResultID(err))
		return
	}
	if err := persistEquipmentElementAttachPlan(plan, p.accountRecord, character, func(next *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, next)
	}); err != nil {
		xlog.GLog.Errorf("persist equipment element attach failed aid:%d character:%d pet:%d equipment:%d matched:%t err:%v", p.aid, plan.characterUUID, plan.petUUID, plan.equipmentUUID, plan.matched, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_EquipmentElementAttachRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendCharacterContainerChangedNotify(gateway, plan.characterUUID, plan.nextCharacter.GetItemBag(), 0)
	p.sendClientRes(gateway, uint32(pb.MsgID_EquipmentElementAttachRes_CMD), xerror.Success.Code(), &pb.EquipmentElementAttachRes{
		CharacterUuid:          plan.characterUUID,
		PetUuid:                plan.petUUID,
		EquipmentUuid:          plan.equipmentUUID,
		Matched:                plan.matched,
		ResultEquipment:        proto.Clone(plan.resultEquipment).(*pb.EquipmentRecord),
		MaterialCostResultList: cloneItemCostResults(plan.materialCosts),
	})
}

func prepareEquipmentElementAttachPlan(accountRecord *pb.AccountRecord, characterRecord *pb.CharacterRecord, petUUID, equipmentUUID uint64, materials []*pb.ItemElement) (*equipmentElementAttachPlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || petUUID == 0 || equipmentUUID == 0 || len(materials) == 0 || len(materials) > gameconfig.TiangongMaximumMaterialTypes {
		return nil, errItemSynthesisInvalidArgument
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Skill == nil || gameconfig.GGameConfig.Tiangong == nil {
		return nil, fmt.Errorf("%w: item, skill or tiangong config is not loaded", errItemSynthesisRecordInvalid)
	}
	if err := validateItemSynthesisPet(characterRecord, petUUID); err != nil {
		return nil, err
	}
	// 附元素不会新增装备, 但按天工加工规则仍要求操作前至少一个空位.
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

	attribute, matched := gameconfig.GGameConfig.Tiangong.MatchAttribute(equipment.GetAssetId(), materialCounts)
	if matched && equipment.GetElementAttribute().GetElement() == attribute.Element {
		return nil, fmt.Errorf("%w: equipment %d already has element %d", errItemSynthesisFailedPrecondition, equipmentUUID, attribute.Element)
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextCharacter := nextAccountRecord.GetCharacterRecordList()[characterSlot]
	nextEquipment := nextCharacter.GetItemBag().GetEquipmentRecordMap()[equipmentUUID]
	if nextEquipment == nil {
		return nil, fmt.Errorf("%w: cloned equipment %d is missing", errItemSynthesisRecordInvalid, equipmentUUID)
	}
	nextItemManager := newCharacterItemManager(nextCharacter)
	for _, material := range materials {
		if err := nextItemManager.Consume(material.GetAssetId(), material.GetCount()); err != nil {
			return nil, fmt.Errorf("%w: consume material %d: %v", errItemSynthesisRecordInvalid, material.GetAssetId(), err)
		}
	}
	if matched {
		nextEquipment.ElementAttribute = &pb.EquipmentElementAttribute{Element: attribute.Element, Value: equipmentAttachedElementValue}
		if err := validateEquipmentRecord(nextEquipment, equipmentUUID); err != nil {
			return nil, fmt.Errorf("%w: attached equipment %d: %v", errItemSynthesisRecordInvalid, equipmentUUID, err)
		}
	}

	plan := &equipmentElementAttachPlan{
		characterUUID:     characterRecord.GetBase().GetUuid(),
		petUUID:           petUUID,
		equipmentUUID:     equipmentUUID,
		matched:           matched,
		resultEquipment:   nextEquipment,
		previousUsedUUID:  accountRecord.GetUsedUuid(),
		characterSlot:     characterSlot,
		previousCharacter: characterRecord,
		nextCharacter:     nextCharacter,
		nextAccountRecord: nextAccountRecord,
	}
	for _, material := range materials {
		plan.materialCosts = append(plan.materialCosts, &pb.ItemCostResult{
			ItemId:         material.GetAssetId(),
			ConsumedCount:  material.GetCount(),
			RemainingCount: nextItemManager.Count(material.GetAssetId()),
		})
	}
	return plan, nil
}

func persistEquipmentElementAttachPlan(plan *equipmentElementAttachPlan, accountRecord *pb.AccountRecord, character *character, persist func(*pb.AccountRecord) error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.nextAccountRecord == nil || plan.nextCharacter == nil || plan.resultEquipment == nil {
		return errItemSynthesisInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) || accountRecord.GetCharacterRecordList()[plan.characterSlot] != plan.previousCharacter || character.record != plan.previousCharacter || accountRecord.GetUsedUuid() != plan.previousUsedUUID {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errItemSynthesisRecordInvalid)
	}
	if err := persist(plan.nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[plan.characterSlot] = plan.nextCharacter
	character.record = plan.nextCharacter
	return nil
}
