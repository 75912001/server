package main

import (
	"errors"
	"fmt"
	"math"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xutil "github.com/75912001/xlib/util"
	"google.golang.org/protobuf/proto"
)

const itemSynthesisSkillID uint32 = 8_100_200

var (
	errItemSynthesisInvalidArgument    = errors.New("invalid item synthesis argument")
	errItemSynthesisTargetNotFound     = errors.New("item synthesis target not found")
	errItemSynthesisFailedPrecondition = errors.New("item synthesis precondition failed")
	errItemSynthesisResourceExhausted  = errors.New("item synthesis resource exhausted")
	errItemSynthesisRecordInvalid      = errors.New("item synthesis record is invalid")
)

type itemSynthesisPlan struct {
	characterUUID          uint64
	petUUID                uint64
	matched                bool
	resultEquipment        *pb.EquipmentRecord
	returnedMaterial       *pb.ItemElement
	materialCostResultList []*pb.ItemCostResult
	previousUsedUUID       uint64
	nextUsedUUID           uint64
	characterSlot          int
	previousCharacter      *pb.CharacterRecord
	nextCharacter          *pb.CharacterRecord
	nextAccountRecord      *pb.AccountRecord
}

func (p *Account) onItemSynthesisReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ItemSynthesisReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if err := validateItemSynthesisCharacterState(character); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), itemSynthesisResultID(err))
		return
	}
	plan, err := prepareItemSynthesisPlan(p.accountRecord, character.record, req.GetPetUuid(), req.GetMaterialList(), randomItemSynthesisIndex)
	if err != nil {
		xlog.GLog.Warnf("item synthesis rejected aid:%d character:%d pet:%d materialKinds:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetPetUuid(), len(req.GetMaterialList()), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), itemSynthesisResultID(err))
		return
	}
	if err := persistItemSynthesisPlan(plan, p.accountRecord, character, func(next *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, next)
	}); err != nil {
		xlog.GLog.Errorf("persist item synthesis failed aid:%d character:%d pet:%d matched:%t err:%v", p.aid, plan.characterUUID, plan.petUUID, plan.matched, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), xerror.Internal.Code())
		return
	}

	notifyUsedUUID := uint64(0)
	if plan.matched {
		notifyUsedUUID = plan.nextUsedUUID
	}
	p.sendCharacterContainerChangedNotify(gateway, plan.characterUUID, plan.nextCharacter.GetItemBag(), notifyUsedUUID)
	response := &pb.ItemSynthesisRes{
		CharacterUuid:          plan.characterUUID,
		PetUuid:                plan.petUUID,
		Matched:                plan.matched,
		MaterialCostResultList: cloneItemCostResults(plan.materialCostResultList),
	}
	if plan.matched {
		response.ResultEquipment = proto.Clone(plan.resultEquipment).(*pb.EquipmentRecord)
		response.UsedUuid = plan.nextUsedUUID
	} else {
		response.ReturnedMaterial = proto.Clone(plan.returnedMaterial).(*pb.ItemElement)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), xerror.Success.Code(), response)
}

func validateItemSynthesisCharacterState(character *character) error {
	if character == nil || character.record == nil {
		return errItemSynthesisTargetNotFound
	}
	if !character.online || character.combatRoom != nil {
		return errItemSynthesisFailedPrecondition
	}
	return nil
}

func randomItemSynthesisIndex(total uint64) uint64 {
	return xutil.RandomU64(0, total-1)
}

func prepareItemSynthesisPlan(accountRecord *pb.AccountRecord, characterRecord *pb.CharacterRecord, petUUID uint64, materials []*pb.ItemElement, randomIndex func(uint64) uint64) (*itemSynthesisPlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || petUUID == 0 || len(materials) == 0 || len(materials) > gameconfig.TiangongMaximumMaterialTypes || randomIndex == nil {
		return nil, errItemSynthesisInvalidArgument
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Tiangong == nil {
		return nil, fmt.Errorf("%w: item or tiangong config is not loaded", errItemSynthesisRecordInvalid)
	}

	if err := validateItemSynthesisPet(characterRecord, petUUID); err != nil {
		return nil, err
	}
	// 必须使用加工前的实际背包占位数判断, 不能依赖本次扣除素材腾出位置.
	if itemContainerCount(characterRecord.GetItemBag()) >= int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return nil, fmt.Errorf("%w: item bag has no empty slot", errItemSynthesisFailedPrecondition)
	}

	materialCounts, totalUnits, _, err := validateItemSynthesisMaterials(characterRecord, materials)
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

	recipes, matched := gameconfig.GGameConfig.Tiangong.MatchCandidates(materialCounts)
	if matched && accountRecord.GetUsedUuid() == math.MaxUint64 {
		return nil, fmt.Errorf("%w: account uuid cursor exhausted", errItemSynthesisResourceExhausted)
	}
	var recipe *gameconfig.TiangongRecipe
	if matched {
		recipe = recipes[0]
		if len(recipes) > 1 {
			draw := randomIndex(uint64(len(recipes)))
			if draw >= uint64(len(recipes)) {
				return nil, fmt.Errorf("%w: random recipe index %d is outside %d", errItemSynthesisRecordInvalid, draw, len(recipes))
			}
			recipe = recipes[draw]
		}
	}
	selectedMaterialID := uint32(0)
	if !matched {
		draw := randomIndex(totalUnits)
		if draw >= totalUnits {
			return nil, fmt.Errorf("%w: random index %d is outside %d", errItemSynthesisRecordInvalid, draw, totalUnits)
		}
		for _, material := range materials {
			if draw < material.GetCount() {
				selectedMaterialID = material.GetAssetId()
				break
			}
			draw -= material.GetCount()
		}
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextCharacter := nextAccountRecord.GetCharacterRecordList()[characterSlot]
	nextItemManager := newCharacterItemManager(nextCharacter)
	for _, material := range materials {
		if err := nextItemManager.Consume(material.GetAssetId(), material.GetCount()); err != nil {
			return nil, fmt.Errorf("%w: consume material %d: %v", errItemSynthesisRecordInvalid, material.GetAssetId(), err)
		}
	}

	plan := &itemSynthesisPlan{
		characterUUID:     characterRecord.GetBase().GetUuid(),
		petUUID:           petUUID,
		matched:           matched,
		previousUsedUUID:  accountRecord.GetUsedUuid(),
		nextUsedUUID:      accountRecord.GetUsedUuid(),
		characterSlot:     characterSlot,
		previousCharacter: characterRecord,
		nextCharacter:     nextCharacter,
		nextAccountRecord: nextAccountRecord,
	}
	if matched {
		plan.nextUsedUUID++
		equipment, err := newEquipmentRecord(plan.nextUsedUUID, recipe.ID)
		if err != nil {
			return nil, fmt.Errorf("%w: create equipment %d: %v", errItemSynthesisRecordInvalid, recipe.ID, err)
		}
		if nextCharacter.ItemBag == nil {
			nextCharacter.ItemBag = &pb.ItemContainerRecord{}
		}
		if nextCharacter.ItemBag.EquipmentRecordMap == nil {
			nextCharacter.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
		}
		if _, exists := nextCharacter.ItemBag.EquipmentRecordMap[equipment.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: equipment uuid %d already exists", errItemSynthesisRecordInvalid, equipment.GetUuid())
		}
		nextCharacter.ItemBag.EquipmentRecordMap[equipment.GetUuid()] = equipment
		nextAccountRecord.UsedUuid = plan.nextUsedUUID
		plan.resultEquipment = equipment
	} else {
		if selectedMaterialID == 0 {
			return nil, fmt.Errorf("%w: returned material was not selected", errItemSynthesisRecordInvalid)
		}
		if err := nextItemManager.Add(selectedMaterialID, 1); err != nil {
			return nil, fmt.Errorf("%w: return material %d: %v", errItemSynthesisRecordInvalid, selectedMaterialID, err)
		}
		plan.returnedMaterial = &pb.ItemElement{AssetId: selectedMaterialID, Count: 1}
	}
	for _, material := range materials {
		plan.materialCostResultList = append(plan.materialCostResultList, &pb.ItemCostResult{
			ItemId:         material.GetAssetId(),
			ConsumedCount:  material.GetCount(),
			RemainingCount: nextItemManager.Count(material.GetAssetId()),
		})
	}
	return plan, nil
}

func validateItemSynthesisPet(characterRecord *pb.CharacterRecord, petUUID uint64) error {
	for _, pet := range characterRecord.GetPetRecordList() {
		if pet.GetUuid() != petUUID {
			continue
		}
		for _, skillID := range pet.GetSkillIdList() {
			if skillID == itemSynthesisSkillID {
				return nil
			}
		}
		return fmt.Errorf("%w: pet %d has not learned processing", errItemSynthesisFailedPrecondition, petUUID)
	}
	return fmt.Errorf("%w: pet %d", errItemSynthesisTargetNotFound, petUUID)
}

func validateItemSynthesisMaterials(characterRecord *pb.CharacterRecord, materials []*pb.ItemElement) (map[uint32]uint64, uint64, *characterItemManager, error) {
	materialCounts := make(map[uint32]uint64, len(materials))
	var totalUnits uint64
	itemManager := newCharacterItemManager(characterRecord)
	for _, material := range materials {
		if material == nil || material.GetAssetId() < uint32(pb.AssetID_AssetIDRange_Item_Material_Start) || material.GetAssetId() > uint32(pb.AssetID_AssetIDRange_Item_Material_End) || material.GetCount() == 0 || material.GetCount() > gameconfig.TiangongMaximumMaterialCount {
			return nil, 0, nil, errItemSynthesisInvalidArgument
		}
		if _, exists := materialCounts[material.GetAssetId()]; exists {
			return nil, 0, nil, fmt.Errorf("%w: duplicate material %d", errItemSynthesisInvalidArgument, material.GetAssetId())
		}
		if gameconfig.GGameConfig.Item.Get(material.GetAssetId()) == nil {
			return nil, 0, nil, fmt.Errorf("%w: material %d", errItemSynthesisTargetNotFound, material.GetAssetId())
		}
		if itemManager.Count(material.GetAssetId()) < material.GetCount() {
			return nil, 0, nil, fmt.Errorf("%w: material %d is insufficient", errItemSynthesisFailedPrecondition, material.GetAssetId())
		}
		materialCounts[material.GetAssetId()] = material.GetCount()
		totalUnits += material.GetCount()
	}
	return materialCounts, totalUnits, itemManager, nil
}

func persistItemSynthesisPlan(plan *itemSynthesisPlan, accountRecord *pb.AccountRecord, character *character, persist func(*pb.AccountRecord) error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.nextAccountRecord == nil || plan.nextCharacter == nil {
		return errItemSynthesisInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) || accountRecord.GetCharacterRecordList()[plan.characterSlot] != plan.previousCharacter || character.record != plan.previousCharacter || accountRecord.GetUsedUuid() != plan.previousUsedUUID {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errItemSynthesisRecordInvalid)
	}
	if err := persist(plan.nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[plan.characterSlot] = plan.nextCharacter
	accountRecord.UsedUuid = plan.nextUsedUUID
	character.record = plan.nextCharacter
	return nil
}

func itemSynthesisResultID(err error) uint32 {
	switch {
	case errors.Is(err, errItemSynthesisInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errItemSynthesisTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errItemSynthesisFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errItemSynthesisResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}
