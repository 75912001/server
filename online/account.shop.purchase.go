package main

import (
	"errors"
	"fmt"
	"math"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errShopPurchaseInvalidArgument    = errors.New("invalid shop purchase argument")
	errShopPurchaseTargetNotFound     = errors.New("shop purchase target not found")
	errShopPurchaseFailedPrecondition = errors.New("shop purchase precondition failed")
	errShopPurchaseResourceExhausted  = errors.New("shop purchase resource exhausted")
	errShopPurchaseRecordInvalid      = errors.New("shop purchase record is invalid")
)

const shopPurchaseMaxQuantity = uint32(pb.CharacterLimit_CharacterLimit_MaxItemBagCount)

// shopPurchasePlan 保存一次购买的完整候选账号快照. cache 成功前不修改在线权威档案.
type shopPurchasePlan struct {
	characterUUID       uint64
	itemID              uint32
	quantity            uint32
	unitCost            uint64
	totalCost           uint64
	remainingStone      uint32
	previousUsedUUID    uint64
	nextUsedUUID        uint64
	equipmentRecordList []*pb.EquipmentRecord
	characterSlot       int
	previousCharacter   *pb.CharacterRecord
	nextCharacter       *pb.CharacterRecord
	nextAccountRecord   *pb.AccountRecord
}

func (p *Account) onShopPurchaseReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ShopPurchaseReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if err := validateShopPurchaseCharacterState(character); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), shopPurchaseResultID(err))
		return
	}

	plan, err := prepareShopPurchasePlan(
		p.accountRecord,
		character.record,
		req.GetItemId(),
		req.GetQuantity(),
		req.GetExpectedUnitCost(),
	)
	if err != nil {
		xlog.GLog.Warnf(
			"shop purchase rejected aid:%d character:%d item:%d quantity:%d expectedUnitCost:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetItemId(),
			req.GetQuantity(),
			req.GetExpectedUnitCost(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), shopPurchaseResultID(err))
		return
	}

	if err := persistShopPurchasePlan(plan, p.accountRecord, character, func(nextAccountRecord *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, nextAccountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist shop purchase failed aid:%d character:%d item:%d quantity:%d unitCost:%d totalCost:%d err:%v",
			p.aid,
			plan.characterUUID,
			plan.itemID,
			plan.quantity,
			plan.unitCost,
			plan.totalCost,
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), xerror.Internal.Code())
		return
	}

	equipmentUUIDStart := plan.equipmentRecordList[0].GetUuid()
	equipmentUUIDEnd := plan.equipmentRecordList[len(plan.equipmentRecordList)-1].GetUuid()
	xlog.GLog.Infof(
		"shop purchase success aid:%d character:%d item:%d quantity:%d unitCost:%d totalCost:%d remainingStone:%d equipmentUUIDStart:%d equipmentUUIDEnd:%d",
		p.aid,
		plan.characterUUID,
		plan.itemID,
		plan.quantity,
		plan.unitCost,
		plan.totalCost,
		plan.remainingStone,
		equipmentUUIDStart,
		equipmentUUIDEnd,
	)

	responseEquipmentRecordList := make([]*pb.EquipmentRecord, 0, len(plan.equipmentRecordList))
	for _, equipmentRecord := range plan.equipmentRecordList {
		responseEquipmentRecordList = append(responseEquipmentRecordList, proto.Clone(equipmentRecord).(*pb.EquipmentRecord))
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), xerror.Success.Code(), &pb.ShopPurchaseRes{
		CharacterUuid:       plan.characterUUID,
		ItemId:              plan.itemID,
		Quantity:            plan.quantity,
		UnitCost:            plan.unitCost,
		TotalCost:           plan.totalCost,
		RemainingStone:      plan.remainingStone,
		UsedUuid:            plan.nextUsedUUID,
		EquipmentRecordList: responseEquipmentRecordList,
	})
}

// validateShopPurchaseCharacterState 统一约束购买只能由已上线且不在战斗中的角色执行.
func validateShopPurchaseCharacterState(character *character) error {
	if character == nil || character.record == nil {
		return errShopPurchaseTargetNotFound
	}
	if !character.online || character.combatRoom != nil {
		return errShopPurchaseFailedPrecondition
	}
	return nil
}

// prepareShopPurchasePlan 在独立账号副本中完成扣币、装备 UUID 分配和背包写入.
func prepareShopPurchasePlan(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	itemID uint32,
	quantity uint32,
	expectedUnitCost uint64,
) (*shopPurchasePlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || itemID == 0 || quantity == 0 || quantity > shopPurchaseMaxQuantity || expectedUnitCost == 0 {
		return nil, errShopPurchaseInvalidArgument
	}
	if itemID < uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Start) || itemID > uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_End) {
		return nil, fmt.Errorf("%w: item %d is not a weapon", errShopPurchaseInvalidArgument, itemID)
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil {
		return nil, fmt.Errorf("%w: item config is not loaded", errShopPurchaseRecordInvalid)
	}
	itemEntry := gameconfig.GGameConfig.Item.Get(itemID)
	if itemEntry == nil {
		return nil, fmt.Errorf("%w: item %d", errShopPurchaseTargetNotFound, itemID)
	}
	if itemEntry.ID == nil || *itemEntry.ID != itemID {
		return nil, fmt.Errorf("%w: item %d config id mismatch", errShopPurchaseRecordInvalid, itemID)
	}
	if itemEntry.Cost == 0 {
		return nil, fmt.Errorf("%w: item %d is not sellable", errShopPurchaseFailedPrecondition, itemID)
	}
	if itemEntry.Cost != expectedUnitCost {
		return nil, fmt.Errorf("%w: item %d unit cost changed from %d to %d", errShopPurchaseFailedPrecondition, itemID, expectedUnitCost, itemEntry.Cost)
	}
	if itemEntry.Cost > math.MaxUint64/uint64(quantity) {
		return nil, fmt.Errorf("%w: item %d total cost overflows uint64", errShopPurchaseRecordInvalid, itemID)
	}
	totalCost := itemEntry.Cost * uint64(quantity)
	if totalCost > uint64(characterRecord.GetStone()) {
		return nil, fmt.Errorf("%w: stone %d is less than total cost %d", errShopPurchaseFailedPrecondition, characterRecord.GetStone(), totalCost)
	}
	if itemContainerCount(characterRecord.GetItemBag())+int(quantity) > int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return nil, fmt.Errorf("%w: item bag has %d records and needs %d slots", errShopPurchaseResourceExhausted, itemContainerCount(characterRecord.GetItemBag()), quantity)
	}
	if accountRecord.GetUsedUuid() > math.MaxUint64-uint64(quantity) {
		return nil, fmt.Errorf("%w: account uuid cursor %d cannot allocate %d records", errShopPurchaseResourceExhausted, accountRecord.GetUsedUuid(), quantity)
	}

	characterSlot := -1
	for index, candidate := range accountRecord.GetCharacterRecordList() {
		if candidate == characterRecord && candidate.GetBase().GetUuid() == characterRecord.GetBase().GetUuid() {
			characterSlot = index
			break
		}
	}
	if characterSlot < 0 {
		return nil, fmt.Errorf("%w: character %d record slot not found", errShopPurchaseRecordInvalid, characterRecord.GetBase().GetUuid())
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextCharacter := nextAccountRecord.GetCharacterRecordList()[characterSlot]
	if nextCharacter.ItemBag == nil {
		nextCharacter.ItemBag = &pb.ItemContainerRecord{}
	}
	if nextCharacter.ItemBag.ItemCountMap == nil {
		nextCharacter.ItemBag.ItemCountMap = make(map[uint32]uint64)
	}
	if nextCharacter.ItemBag.EquipmentRecordMap == nil {
		nextCharacter.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}

	previousUsedUUID := accountRecord.GetUsedUuid()
	nextUsedUUID := previousUsedUUID + uint64(quantity)
	equipmentRecordList := make([]*pb.EquipmentRecord, 0, quantity)
	for offset := uint64(1); offset <= uint64(quantity); offset++ {
		equipmentUUID := previousUsedUUID + offset
		if _, exists := nextCharacter.ItemBag.EquipmentRecordMap[equipmentUUID]; exists {
			return nil, fmt.Errorf("%w: equipment uuid %d already exists in item bag", errShopPurchaseRecordInvalid, equipmentUUID)
		}
		equipmentRecord := &pb.EquipmentRecord{Uuid: equipmentUUID, AssetId: itemID}
		nextCharacter.ItemBag.EquipmentRecordMap[equipmentUUID] = equipmentRecord
		equipmentRecordList = append(equipmentRecordList, equipmentRecord)
	}
	nextCharacter.Stone = uint32(uint64(characterRecord.GetStone()) - totalCost)
	nextAccountRecord.UsedUuid = nextUsedUUID

	return &shopPurchasePlan{
		characterUUID:       characterRecord.GetBase().GetUuid(),
		itemID:              itemID,
		quantity:            quantity,
		unitCost:            itemEntry.Cost,
		totalCost:           totalCost,
		remainingStone:      nextCharacter.GetStone(),
		previousUsedUUID:    previousUsedUUID,
		nextUsedUUID:        nextUsedUUID,
		equipmentRecordList: equipmentRecordList,
		characterSlot:       characterSlot,
		previousCharacter:   characterRecord,
		nextCharacter:       nextCharacter,
		nextAccountRecord:   nextAccountRecord,
	}, nil
}

// persistShopPurchasePlan 先持久化完整候选账号快照, 成功后再一次性提交在线内存引用.
func persistShopPurchasePlan(
	plan *shopPurchasePlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func(*pb.AccountRecord) error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.nextAccountRecord == nil || plan.nextCharacter == nil {
		return errShopPurchaseInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) || accountRecord.GetCharacterRecordList()[plan.characterSlot] != plan.previousCharacter || character.record != plan.previousCharacter || accountRecord.GetUsedUuid() != plan.previousUsedUUID {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errShopPurchaseRecordInvalid)
	}
	if err := persist(plan.nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[plan.characterSlot] = plan.nextCharacter
	accountRecord.UsedUuid = plan.nextUsedUUID
	character.record = plan.nextCharacter
	return nil
}

func shopPurchaseResultID(err error) uint32 {
	switch {
	case errors.Is(err, errShopPurchaseInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errShopPurchaseTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errShopPurchaseFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errShopPurchaseResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}
