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
	errItemWarehouseInvalidArgument    = errors.New("invalid item warehouse argument")
	errItemWarehouseTargetNotFound     = errors.New("item warehouse target not found")
	errItemWarehouseFailedPrecondition = errors.New("item warehouse precondition failed")
	errItemWarehouseResourceExhausted  = errors.New("item warehouse capacity exhausted")
	errItemWarehouseRecordInvalid      = errors.New("item warehouse record is invalid")
)

type itemWarehouseTransferPlan struct {
	source          *pb.ItemContainerRecord
	target          *pb.ItemContainerRecord
	stack           *pb.ItemStackTransfer
	equipment       *pb.EquipmentRecord
	equipmentUUID   uint64
	sourceItemCount uint64
	targetItemCount uint64
}

func (p *Account) onItemWarehouseDepositReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ItemWarehouseDepositReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.itemWarehouseCharacter(req.GetCharacterUuid())
	if character == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := prepareItemWarehouseTransfer(
		character.record.GetItemBag(),
		p.accountRecord.GetItemWarehouse(),
		req.GetStack(),
		req.GetEquipmentUuid(),
		int(pb.AccountRecordLimit_AccountRecordLimit_MaxItemWarehouseCount),
	)
	if err != nil {
		p.logItemWarehouseFailure("deposit", req.GetCharacterUuid(), req.GetStack(), req.GetEquipmentUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), itemWarehouseResultID(err))
		return
	}
	if err := persistItemWarehouseTransfer(plan, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist item warehouse deposit failed aid:%d character:%d err:%v", p.aid, req.GetCharacterUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), xerror.Internal.Code())
		return
	}

	res := &pb.ItemWarehouseDepositRes{CharacterUuid: req.GetCharacterUuid()}
	if plan.stack != nil {
		res.Item = &pb.ItemWarehouseDepositRes_Stack{Stack: proto.Clone(plan.stack).(*pb.ItemStackTransfer)}
	} else {
		res.Item = &pb.ItemWarehouseDepositRes_EquipmentUuid{EquipmentUuid: plan.equipmentUUID}
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_ItemWarehouseDepositRes_CMD), xerror.Success.Code(), res)
}

func (p *Account) onItemWarehouseWithdrawReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ItemWarehouseWithdrawReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.itemWarehouseCharacter(req.GetCharacterUuid())
	if character == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := prepareItemWarehouseTransfer(
		p.accountRecord.GetItemWarehouse(),
		character.record.GetItemBag(),
		req.GetStack(),
		req.GetEquipmentUuid(),
		int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount),
	)
	if err != nil {
		p.logItemWarehouseFailure("withdraw", req.GetCharacterUuid(), req.GetStack(), req.GetEquipmentUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), itemWarehouseResultID(err))
		return
	}
	if err := persistItemWarehouseTransfer(plan, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist item warehouse withdraw failed aid:%d character:%d err:%v", p.aid, req.GetCharacterUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), xerror.Internal.Code())
		return
	}

	res := &pb.ItemWarehouseWithdrawRes{CharacterUuid: req.GetCharacterUuid()}
	if plan.stack != nil {
		res.Item = &pb.ItemWarehouseWithdrawRes_Stack{Stack: proto.Clone(plan.stack).(*pb.ItemStackTransfer)}
	} else {
		res.Item = &pb.ItemWarehouseWithdrawRes_EquipmentUuid{EquipmentUuid: plan.equipmentUUID}
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_ItemWarehouseWithdrawRes_CMD), xerror.Success.Code(), res)
}

func (p *Account) itemWarehouseCharacter(characterUUID uint64) *character {
	if p == nil || p.characterManager == nil {
		return nil
	}
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		return nil
	}
	return character
}

func prepareItemWarehouseTransfer(
	source *pb.ItemContainerRecord,
	target *pb.ItemContainerRecord,
	stack *pb.ItemStackTransfer,
	equipmentUUID uint64,
	targetCapacity int,
) (*itemWarehouseTransferPlan, error) {
	if source == nil || target == nil || targetCapacity <= 0 {
		return nil, errItemWarehouseInvalidArgument
	}
	if (stack == nil) == (equipmentUUID == 0) {
		return nil, fmt.Errorf("%w: exactly one item type is required", errItemWarehouseInvalidArgument)
	}

	plan := &itemWarehouseTransferPlan{source: source, target: target}
	if stack != nil {
		itemID := stack.GetAssetId()
		count := stack.GetCount()
		if itemID == 0 || count == 0 {
			return nil, errItemWarehouseInvalidArgument
		}
		if isCharacterAssetItemID(itemID) {
			return nil, fmt.Errorf("%w: character asset %d cannot be transferred", errItemWarehouseInvalidArgument, itemID)
		}
		if _, err := configuredItem(itemID); err != nil {
			return nil, err
		}
		sourceCount := source.GetItemCountMap()[itemID]
		if sourceCount < count {
			return nil, fmt.Errorf("%w: item %d count %d is less than %d", errItemWarehouseFailedPrecondition, itemID, sourceCount, count)
		}
		targetCount := target.GetItemCountMap()[itemID]
		if count > math.MaxUint64-targetCount {
			return nil, fmt.Errorf("%w: item %d target count overflows uint64", errItemWarehouseFailedPrecondition, itemID)
		}
		if _, exists := target.GetItemCountMap()[itemID]; !exists && itemContainerCount(target) >= targetCapacity {
			return nil, fmt.Errorf("%w: target count %d", errItemWarehouseResourceExhausted, itemContainerCount(target))
		}
		plan.stack = proto.Clone(stack).(*pb.ItemStackTransfer)
		plan.sourceItemCount = sourceCount
		plan.targetItemCount = targetCount
		return plan, nil
	}

	equipment := source.GetEquipmentRecordMap()[equipmentUUID]
	if equipment == nil {
		return nil, fmt.Errorf("%w: equipment %d", errItemWarehouseTargetNotFound, equipmentUUID)
	}
	if equipment.GetUuid() != equipmentUUID {
		return nil, fmt.Errorf("%w: equipment key %d does not match uuid %d", errItemWarehouseRecordInvalid, equipmentUUID, equipment.GetUuid())
	}
	if _, exists := target.GetEquipmentRecordMap()[equipmentUUID]; exists {
		return nil, fmt.Errorf("%w: equipment %d already exists in target", errItemWarehouseRecordInvalid, equipmentUUID)
	}
	if err := configuredEquipment(equipment.GetAssetId()); err != nil {
		return nil, err
	}
	if itemContainerCount(target) >= targetCapacity {
		return nil, fmt.Errorf("%w: target count %d", errItemWarehouseResourceExhausted, itemContainerCount(target))
	}
	plan.equipment = equipment
	plan.equipmentUUID = equipmentUUID
	return plan, nil
}

func configuredEquipment(equipmentID uint32) error {
	if equipmentID < uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Start) ||
		equipmentID > uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_End) {
		return fmt.Errorf("%w: equipment id %d is invalid", errItemWarehouseRecordInvalid, equipmentID)
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Item.Get(equipmentID) == nil {
		return fmt.Errorf("%w: equipment config %d not found", errItemWarehouseTargetNotFound, equipmentID)
	}
	return nil
}

// persistItemWarehouseTransfer 只在计划完成全部本地校验后修改两个容器, cache 失败时恢复原值.
func persistItemWarehouseTransfer(plan *itemWarehouseTransferPlan, persist func() error) error {
	if plan == nil || plan.source == nil || plan.target == nil || persist == nil {
		return errItemWarehouseInvalidArgument
	}
	if plan.stack != nil {
		itemID := plan.stack.GetAssetId()
		count := plan.stack.GetCount()
		if plan.source.ItemCountMap == nil {
			plan.source.ItemCountMap = make(map[uint32]uint64)
		}
		if plan.target.ItemCountMap == nil {
			plan.target.ItemCountMap = make(map[uint32]uint64)
		}
		setItemContainerCount(plan.source.ItemCountMap, itemID, plan.sourceItemCount-count)
		setItemContainerCount(plan.target.ItemCountMap, itemID, plan.targetItemCount+count)
		if err := persist(); err != nil {
			setItemContainerCount(plan.source.ItemCountMap, itemID, plan.sourceItemCount)
			setItemContainerCount(plan.target.ItemCountMap, itemID, plan.targetItemCount)
			return err
		}
		return nil
	}

	if plan.equipment == nil || plan.equipmentUUID == 0 {
		return errItemWarehouseInvalidArgument
	}
	if plan.source.EquipmentRecordMap == nil {
		plan.source.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}
	if plan.target.EquipmentRecordMap == nil {
		plan.target.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}
	delete(plan.source.EquipmentRecordMap, plan.equipmentUUID)
	plan.target.EquipmentRecordMap[plan.equipmentUUID] = plan.equipment
	if err := persist(); err != nil {
		delete(plan.target.EquipmentRecordMap, plan.equipmentUUID)
		plan.source.EquipmentRecordMap[plan.equipmentUUID] = plan.equipment
		return err
	}
	return nil
}

func setItemContainerCount(itemCountMap map[uint32]uint64, itemID uint32, count uint64) {
	if count == 0 {
		delete(itemCountMap, itemID)
		return
	}
	itemCountMap[itemID] = count
}

func itemWarehouseResultID(err error) uint32 {
	switch {
	case errors.Is(err, errItemWarehouseInvalidArgument), errors.Is(err, errItemUseInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errItemWarehouseTargetNotFound), errors.Is(err, errItemUseTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errItemWarehouseFailedPrecondition), errors.Is(err, errItemUseFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errItemWarehouseResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}

func (p *Account) logItemWarehouseFailure(action string, characterUUID uint64, stack *pb.ItemStackTransfer, equipmentUUID uint64, err error) {
	if stack != nil {
		xlog.GLog.Warnf("prepare item warehouse %s failed aid:%d character:%d item:%d count:%d err:%v", action, p.aid, characterUUID, stack.GetAssetId(), stack.GetCount(), err)
		return
	}
	xlog.GLog.Warnf("prepare item warehouse %s failed aid:%d character:%d equipment:%d err:%v", action, p.aid, characterUUID, equipmentUUID, err)
}
