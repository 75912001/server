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
	characterUUID uint64
	petUUID       uint64
	matched       bool
	characterSlot int
	materials     []*pb.ItemElement
	// nextUsedUUID 与 newEquipment 只在命中配方时有效, 装备在 prepare 阶段建好、apply 阶段插入.
	nextUsedUUID uint64
	newEquipment *pb.EquipmentRecord
	// selectedMaterialID 只在未命中配方时有效, 表示返还哪种素材.
	selectedMaterialID uint32
	// materialCostResultList 在 apply 阶段按消耗后的权威数量填充, 供响应返回.
	materialCostResultList []*pb.ItemCostResult
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
	if err := applyItemSynthesisPlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf("persist item synthesis failed aid:%d character:%d pet:%d matched:%t err:%v", p.aid, plan.characterUUID, plan.petUUID, plan.matched, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemSynthesisRes_CMD), xerror.Internal.Code())
		return
	}

	notifyUsedUUID := uint64(0)
	if plan.matched {
		notifyUsedUUID = plan.nextUsedUUID
	}
	p.sendCharacterContainerChangedNotify(gateway, plan.characterUUID, character.record.GetItemBag(), notifyUsedUUID)
	response := &pb.ItemSynthesisRes{
		CharacterUuid:          plan.characterUUID,
		PetUuid:                plan.petUUID,
		Matched:                plan.matched,
		MaterialCostResultList: cloneItemCostResults(plan.materialCostResultList),
	}
	if plan.matched {
		response.ResultEquipment = proto.Clone(plan.newEquipment).(*pb.EquipmentRecord)
		response.UsedUuid = plan.nextUsedUUID
	} else {
		response.ReturnedMaterial = &pb.ItemElement{AssetId: plan.selectedMaterialID, Count: 1}
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

	nextUsedUUID := accountRecord.GetUsedUuid()
	var newEquipment *pb.EquipmentRecord
	if matched {
		nextUsedUUID++
		// 新装备是全新对象而非克隆, 因此在 prepare 阶段构建, apply 阶段只做插入.
		equipment, err := newEquipmentRecord(nextUsedUUID, recipe.ID)
		if err != nil {
			return nil, fmt.Errorf("%w: create equipment %d: %v", errItemSynthesisRecordInvalid, recipe.ID, err)
		}
		if _, exists := characterRecord.GetItemBag().GetEquipmentRecordMap()[equipment.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: equipment uuid %d already exists", errItemSynthesisRecordInvalid, equipment.GetUuid())
		}
		newEquipment = equipment
	} else if selectedMaterialID == 0 {
		return nil, fmt.Errorf("%w: returned material was not selected", errItemSynthesisRecordInvalid)
	}

	return &itemSynthesisPlan{
		characterUUID:      characterRecord.GetBase().GetUuid(),
		petUUID:            petUUID,
		matched:            matched,
		characterSlot:      characterSlot,
		materials:          materials,
		nextUsedUUID:       nextUsedUUID,
		newEquipment:       newEquipment,
		selectedMaterialID: selectedMaterialID,
	}, nil
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

// applyItemSynthesisPlan 把计划原地应用到权威账号档案, 再通知落盘。
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知。
func applyItemSynthesisPlan(plan *itemSynthesisPlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errItemSynthesisInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != character.record {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errItemSynthesisRecordInvalid)
	}

	itemManager := newCharacterItemManager(character.record)
	for _, material := range plan.materials {
		if err := itemManager.Consume(material.GetAssetId(), material.GetCount()); err != nil {
			return fmt.Errorf("%w: consume material %d: %v", errItemSynthesisRecordInvalid, material.GetAssetId(), err)
		}
	}
	if plan.matched {
		if character.record.ItemBag == nil {
			character.record.ItemBag = &pb.ItemContainerRecord{}
		}
		if character.record.ItemBag.EquipmentRecordMap == nil {
			character.record.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
		}
		if _, exists := character.record.ItemBag.EquipmentRecordMap[plan.newEquipment.GetUuid()]; exists {
			return fmt.Errorf("%w: equipment uuid %d already exists", errItemSynthesisRecordInvalid, plan.newEquipment.GetUuid())
		}
		character.record.ItemBag.EquipmentRecordMap[plan.newEquipment.GetUuid()] = plan.newEquipment
		accountRecord.UsedUuid = plan.nextUsedUUID
	} else {
		if err := itemManager.Add(plan.selectedMaterialID, 1); err != nil {
			return fmt.Errorf("%w: return material %d: %v", errItemSynthesisRecordInvalid, plan.selectedMaterialID, err)
		}
	}

	plan.materialCostResultList = plan.materialCostResultList[:0]
	for _, material := range plan.materials {
		plan.materialCostResultList = append(plan.materialCostResultList, &pb.ItemCostResult{
			ItemId:         material.GetAssetId(),
			ConsumedCount:  material.GetCount(),
			RemainingCount: itemManager.Count(material.GetAssetId()),
		})
	}
	return persist()
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
