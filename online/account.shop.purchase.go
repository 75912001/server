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

// shopPurchaseMaxQuantity 单次购买份数上限(协议范围 1-30).
const shopPurchaseMaxQuantity = uint32(pb.CharacterLimit_CharacterLimit_MaxItemBagCount)

// shopPurchasePlan 保存一次购买的完整变更计划. 校验阶段不修改在线权威档案.
type shopPurchasePlan struct {
	characterUUID       uint64
	itemID              uint32
	itemCount           uint32
	quantity            uint32
	isEquipment         bool
	totalDelivered      uint64
	costAmounts         []storeCostAmount
	nextUsedUUID        uint64
	equipmentRecordList []*pb.EquipmentRecord
	characterSlot       int
	// costResultList 与 itemRemainingCount 在 apply 阶段按消耗后的权威数量填充, 供响应返回.
	costResultList     []*pb.ItemCostResult
	itemRemainingCount uint64
}

func (p *Account) onShopPurchaseReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ShopPurchaseReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetItemId() == 0 {
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
	)
	if err != nil {
		xlog.GLog.Warnf(
			"shop purchase rejected aid:%d character:%d item:%d quantity:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetItemId(),
			req.GetQuantity(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), shopPurchaseResultID(err))
		return
	}

	if err := applyShopPurchasePlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf(
			"persist shop purchase failed aid:%d character:%d item:%d quantity:%d costKinds:%d err:%v",
			p.aid,
			plan.characterUUID,
			plan.itemID,
			plan.quantity,
			len(plan.costResultList),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), xerror.Internal.Code())
		return
	}

	xlog.GLog.Infof(
		"shop purchase success aid:%d character:%d item:%d itemCount:%d quantity:%d costKinds:%d equipmentCount:%d itemRemainingCount:%d",
		p.aid,
		plan.characterUUID,
		plan.itemID,
		plan.itemCount,
		plan.quantity,
		len(plan.costResultList),
		len(plan.equipmentRecordList),
		plan.itemRemainingCount,
	)

	responseEquipmentRecordList := make([]*pb.EquipmentRecord, 0, len(plan.equipmentRecordList))
	for _, equipmentRecord := range plan.equipmentRecordList {
		responseEquipmentRecordList = append(responseEquipmentRecordList, proto.Clone(equipmentRecord).(*pb.EquipmentRecord))
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_ShopPurchaseRes_CMD), xerror.Success.Code(), &pb.ShopPurchaseRes{
		CharacterUuid:       plan.characterUUID,
		ItemId:              plan.itemID,
		ItemCount:           plan.itemCount,
		Quantity:            plan.quantity,
		UsedUuid:            plan.nextUsedUUID,
		CostResultList:      cloneItemCostResults(plan.costResultList),
		EquipmentRecordList: responseEquipmentRecordList,
		ItemRemainingCount:  plan.itemRemainingCount,
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

// storeEntryInt 安全解引用商店配置数值字段.
func storeEntryUint32(value *uint32) uint32 {
	if value == nil {
		return 0
	}
	return *value
}

// prepareShopPurchasePlan 依据 商店.yaml 在独立账号副本中完成多资源扣除、装备 UUID 分配和背包写入.
func prepareShopPurchasePlan(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	itemID uint32,
	quantity uint32,
) (*shopPurchasePlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || itemID == 0 || quantity == 0 || quantity > shopPurchaseMaxQuantity {
		return nil, errShopPurchaseInvalidArgument
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Store == nil || gameconfig.GGameConfig.Item == nil {
		return nil, fmt.Errorf("%w: store or item config is not loaded", errShopPurchaseRecordInvalid)
	}
	storeEntry, exists := gameconfig.GGameConfig.Store.GetItemEntryByID(itemID)
	if !exists || storeEntry == nil {
		return nil, fmt.Errorf("%w: store item %d", errShopPurchaseTargetNotFound, itemID)
	}
	itemCount := storeEntryUint32(storeEntry.ItemCount)
	if itemCount == 0 || storeEntry.Costs == nil {
		return nil, fmt.Errorf("%w: store item %d has incomplete fields", errShopPurchaseRecordInvalid, itemID)
	}
	itemEntry := gameconfig.GGameConfig.Item.Get(itemID)
	if itemEntry == nil {
		return nil, fmt.Errorf("%w: item %d", errShopPurchaseTargetNotFound, itemID)
	}
	if itemEntry.ID == nil || *itemEntry.ID != itemID {
		return nil, fmt.Errorf("%w: store item %d item config id mismatch", errShopPurchaseRecordInvalid, itemID)
	}
	costAmounts, err := prepareStoreCostAmounts(storeEntry, quantity, characterRecord)
	if err != nil {
		if errors.Is(err, errStoreCostInsufficient) {
			return nil, fmt.Errorf("%w: %v", errShopPurchaseFailedPrecondition, err)
		}
		return nil, fmt.Errorf("%w: %v", errShopPurchaseRecordInvalid, err)
	}
	totalDelivered := uint64(itemCount) * uint64(quantity)
	isEquipment := itemID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Start) && itemID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_End)
	if isEquipment {
		if uint64(itemContainerCount(characterRecord.GetItemBag()))+totalDelivered > uint64(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
			return nil, fmt.Errorf("%w: item bag has %d records and needs %d slots", errShopPurchaseResourceExhausted, itemContainerCount(characterRecord.GetItemBag()), totalDelivered)
		}
		if accountRecord.GetUsedUuid() > math.MaxUint64-totalDelivered {
			return nil, fmt.Errorf("%w: account uuid cursor %d cannot allocate %d records", errShopPurchaseResourceExhausted, accountRecord.GetUsedUuid(), totalDelivered)
		}
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

	previousUsedUUID := accountRecord.GetUsedUuid()
	nextUsedUUID := previousUsedUUID
	equipmentRecordList := make([]*pb.EquipmentRecord, 0)
	if isEquipment {
		nextUsedUUID += totalDelivered
		equipmentRecordList = make([]*pb.EquipmentRecord, 0, totalDelivered)
		for offset := uint64(1); offset <= totalDelivered; offset++ {
			equipmentUUID := previousUsedUUID + offset
			if _, exists := characterRecord.GetItemBag().GetEquipmentRecordMap()[equipmentUUID]; exists {
				return nil, fmt.Errorf("%w: equipment uuid %d already exists in item bag", errShopPurchaseRecordInvalid, equipmentUUID)
			}
			// 新装备是全新对象而非克隆, 因此在这里构建, apply 阶段只做插入.
			equipmentRecord, err := newEquipmentRecord(equipmentUUID, itemID)
			if err != nil {
				return nil, fmt.Errorf("%w: create equipment %d: %v", errShopPurchaseRecordInvalid, equipmentUUID, err)
			}
			equipmentRecordList = append(equipmentRecordList, equipmentRecord)
		}
	} else if err := validateShopPurchaseBagDelivery(characterRecord, itemID, costAmounts); err != nil {
		return nil, err
	}

	return &shopPurchasePlan{
		characterUUID:       characterRecord.GetBase().GetUuid(),
		itemID:              itemID,
		itemCount:           itemCount,
		quantity:            quantity,
		isEquipment:         isEquipment,
		totalDelivered:      totalDelivered,
		costAmounts:         costAmounts,
		nextUsedUUID:        nextUsedUUID,
		equipmentRecordList: equipmentRecordList,
		characterSlot:       characterSlot,
	}, nil
}

// validateShopPurchaseBagDelivery 预判非装备交付是否需要新堆叠, 使 apply 阶段的 Add 不可能失败.
// 成本消耗可能腾出槽位, 因此按"扣除成本后的堆叠数"判断, 避免比改造前更早地拒绝购买.
func validateShopPurchaseBagDelivery(characterRecord *pb.CharacterRecord, itemID uint32, costAmounts []storeCostAmount) error {
	if isCharacterAssetItemID(itemID) {
		return nil
	}
	if _, exists := characterRecord.GetItemBag().GetItemCountMap()[itemID]; exists {
		return nil
	}
	itemManager := newCharacterItemManager(characterRecord)
	freed := uint64(0)
	for _, amount := range costAmounts {
		if amount.itemID == itemID || isCharacterAssetItemID(amount.itemID) {
			continue
		}
		if itemManager.Count(amount.itemID) == amount.count {
			freed++
		}
	}
	current := uint64(itemContainerCount(characterRecord.GetItemBag()))
	if current < freed {
		freed = current
	}
	if current-freed >= uint64(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return fmt.Errorf("%w: item bag has no empty slot", errShopPurchaseResourceExhausted)
	}
	return nil
}

// applyShopPurchasePlan 把计划原地应用到权威账号档案, 再通知落盘。
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知。
func applyShopPurchasePlan(plan *shopPurchasePlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errShopPurchaseInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != character.record {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errShopPurchaseRecordInvalid)
	}
	if character.record.ItemBag == nil {
		character.record.ItemBag = &pb.ItemContainerRecord{}
	}
	if character.record.ItemBag.ItemCountMap == nil {
		character.record.ItemBag.ItemCountMap = make(map[uint32]uint64)
	}
	if character.record.ItemBag.EquipmentRecordMap == nil {
		character.record.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}

	costResultList, err := consumeStoreCostAmounts(character.record, plan.costAmounts)
	if err != nil {
		return fmt.Errorf("%w: %v", errShopPurchaseRecordInvalid, err)
	}
	plan.costResultList = costResultList

	if plan.isEquipment {
		for _, equipmentRecord := range plan.equipmentRecordList {
			character.record.ItemBag.EquipmentRecordMap[equipmentRecord.GetUuid()] = equipmentRecord
		}
	} else {
		itemManager := newCharacterItemManager(character.record)
		if err := itemManager.Add(plan.itemID, plan.totalDelivered); err != nil {
			return fmt.Errorf("%w: deliver item %d: %v", errShopPurchaseRecordInvalid, plan.itemID, err)
		}
		plan.itemRemainingCount = itemManager.Count(plan.itemID)
	}
	accountRecord.UsedUuid = plan.nextUsedUUID
	return persist()
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
