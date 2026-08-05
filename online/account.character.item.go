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
	errItemUseInvalidArgument    = errors.New("invalid item use argument")
	errItemUseTargetNotFound     = errors.New("item use target not found")
	errItemUseFailedPrecondition = errors.New("item use precondition failed")
	errItemUseRecordInvalid      = errors.New("item record is invalid")
)

const petMaxLoyalty uint32 = 100

type characterItemManager struct {
	record *pb.CharacterRecord
}

func newCharacterItemManager(record *pb.CharacterRecord) *characterItemManager {
	return &characterItemManager{record: record}
}

func (p *characterItemManager) Count(itemID uint32) uint64 {
	if p == nil || p.record == nil || p.record.GetItemBag() == nil {
		return 0
	}
	return p.record.GetItemBag().GetItemCountMap()[itemID]
}

func (p *characterItemManager) Add(itemID uint32, count uint64) error {
	if _, err := configuredItem(itemID); err != nil {
		return err
	}
	if p == nil || p.record == nil || count == 0 {
		return fmt.Errorf("%w: item manager or count is invalid", errItemUseInvalidArgument)
	}
	current := p.Count(itemID)
	if count > math.MaxUint64-current {
		return fmt.Errorf("%w: item %d count overflows uint64", errItemUseFailedPrecondition, itemID)
	}
	if p.record.ItemBag == nil {
		p.record.ItemBag = &pb.ItemContainerRecord{}
	}
	if p.record.ItemBag.ItemCountMap == nil {
		p.record.ItemBag.ItemCountMap = make(map[uint32]uint64)
	}
	if _, exists := p.record.ItemBag.ItemCountMap[itemID]; !exists &&
		itemContainerCount(p.record.ItemBag) >= int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return fmt.Errorf("%w: item bag capacity exhausted", errItemUseFailedPrecondition)
	}
	p.record.ItemBag.ItemCountMap[itemID] = current + count
	return nil
}

func (p *characterItemManager) Consume(itemID uint32, count uint64) error {
	if _, err := configuredItem(itemID); err != nil {
		return err
	}
	if p == nil || p.record == nil || count == 0 {
		return fmt.Errorf("%w: item manager or count is invalid", errItemUseInvalidArgument)
	}
	current := p.Count(itemID)
	if current < count {
		return fmt.Errorf("%w: item %d count %d is less than %d", errItemUseFailedPrecondition, itemID, current, count)
	}
	if current == count {
		delete(p.record.GetItemBag().ItemCountMap, itemID)
		return nil
	}
	p.record.GetItemBag().ItemCountMap[itemID] = current - count
	return nil
}

func configuredItem(itemID uint32) (*gameconfig.ItemEntry, error) {
	if itemID < uint32(pb.AssetIDRange_AssetIDRange_Item_Item_Start) ||
		itemID > uint32(pb.AssetIDRange_AssetIDRange_Item_Item_End) ||
		gameconfig.GGameConfig == nil ||
		gameconfig.GGameConfig.Item == nil {
		return nil, fmt.Errorf("%w: item config is not loaded or item id is empty", errItemUseInvalidArgument)
	}
	entry := gameconfig.GGameConfig.Item.Get(itemID)
	if entry == nil {
		return nil, fmt.Errorf("%w: item config %d not found", errItemUseTargetNotFound, itemID)
	}
	return entry, nil
}

func itemContainerCount(container *pb.ItemContainerRecord) int {
	if container == nil {
		return 0
	}
	return len(container.GetItemCountMap()) + len(container.GetEquipmentRecordMap())
}

type characterItemUsePlan struct {
	characterUUID    uint64
	itemID           uint32
	targetPetUUID    uint64
	previous         *pb.CharacterRecord
	next             *pb.CharacterRecord
	characterChanged bool
	petChangedUUIDs  []uint64
}

func prepareCharacterItemUsePlan(record *pb.CharacterRecord, itemID uint32, targetPetUUID uint64) (*characterItemUsePlan, error) {
	if record == nil || record.GetBase().GetUuid() == 0 || itemID == 0 {
		return nil, fmt.Errorf("%w: character or item id is empty", errItemUseInvalidArgument)
	}
	entry, err := configuredItem(itemID)
	if err != nil {
		return nil, err
	}
	if entry.Use == nil || entry.Use.Target == nil || (entry.Use.Exp == nil) == (entry.Use.Loyalty == nil) {
		return nil, fmt.Errorf("%w: item %d use config is incomplete", errItemUseRecordInvalid, itemID)
	}
	if newCharacterItemManager(record).Count(itemID) == 0 {
		return nil, fmt.Errorf("%w: item %d is not owned", errItemUseFailedPrecondition, itemID)
	}

	next := proto.Clone(record).(*pb.CharacterRecord)
	plan := &characterItemUsePlan{
		characterUUID: record.GetBase().GetUuid(),
		itemID:        itemID,
		targetPetUUID: targetPetUUID,
		previous:      record,
		next:          next,
	}
	switch *entry.Use.Target {
	case gameconfig.ItemUseTargetCharacter:
		if targetPetUUID != 0 || entry.Use.Exp == nil || *entry.Use.Exp == 0 {
			return nil, fmt.Errorf("%w: character item cannot target pet %d", errItemUseInvalidArgument, targetPetUUID)
		}
		settlement, err := applyCharacterExperience(next, *entry.Use.Exp)
		if err != nil {
			return nil, fmt.Errorf("%w: apply character experience: %v", errItemUseRecordInvalid, err)
		}
		if settlement.AppliedExp == 0 {
			return nil, fmt.Errorf("%w: character is at maximum experience", errItemUseFailedPrecondition)
		}
		plan.characterChanged = true
	case gameconfig.ItemUseTargetPet:
		if targetPetUUID == 0 {
			return nil, fmt.Errorf("%w: pet target is empty", errItemUseInvalidArgument)
		}
		var targetPet *pb.PetRecord
		for _, petRecord := range next.GetPetRecordList() {
			if petRecord != nil && petRecord.GetUuid() == targetPetUUID {
				targetPet = petRecord
				break
			}
		}
		if targetPet == nil {
			return nil, fmt.Errorf("%w: carried pet %d", errItemUseTargetNotFound, targetPetUUID)
		}
		if entry.Use.Exp != nil && *entry.Use.Exp > 0 {
			settlement, err := applyPetExperience(targetPet, *entry.Use.Exp)
			if err != nil {
				return nil, fmt.Errorf("%w: apply pet experience: %v", errItemUseRecordInvalid, err)
			}
			if settlement.AppliedExp == 0 {
				return nil, fmt.Errorf("%w: pet %d is at maximum experience", errItemUseFailedPrecondition, targetPetUUID)
			}
		} else if entry.Use.Loyalty != nil && *entry.Use.Loyalty > 0 {
			if targetPet.GetLoyalty() >= petMaxLoyalty {
				return nil, fmt.Errorf("%w: pet %d is at maximum loyalty", errItemUseFailedPrecondition, targetPetUUID)
			}
			remainingLoyalty := petMaxLoyalty - targetPet.GetLoyalty()
			if *entry.Use.Loyalty >= remainingLoyalty {
				targetPet.Loyalty = petMaxLoyalty
			} else {
				targetPet.Loyalty += *entry.Use.Loyalty
			}
		} else {
			return nil, fmt.Errorf("%w: item %d pet effect is invalid", errItemUseRecordInvalid, itemID)
		}
		plan.petChangedUUIDs = []uint64{targetPetUUID}
	default:
		return nil, fmt.Errorf("%w: item %d target %q", errItemUseRecordInvalid, itemID, *entry.Use.Target)
	}
	if err := newCharacterItemManager(next).Consume(itemID, 1); err != nil {
		return nil, err
	}
	return plan, nil
}

func persistCharacterItemUsePlan(
	plan *characterItemUsePlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func() error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil {
		return fmt.Errorf("item use persistence input is nil")
	}
	slot := -1
	for index, record := range accountRecord.GetCharacterRecordList() {
		if record == plan.previous && record.GetBase().GetUuid() == plan.characterUUID {
			slot = index
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("character %d record slot not found", plan.characterUUID)
	}
	accountRecord.CharacterRecordList[slot] = plan.next
	if err := persist(); err != nil {
		accountRecord.CharacterRecordList[slot] = plan.previous
		return err
	}
	character.record = plan.next
	return nil
}

func (p *Account) onItemUseReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.ItemUseReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetItemId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemUseRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemUseRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemUseRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	plan, err := prepareCharacterItemUsePlan(character.record, req.GetItemId(), req.GetTargetPetUuid())
	if err != nil {
		resultID := xerror.Internal.Code()
		switch {
		case errors.Is(err, errItemUseInvalidArgument):
			resultID = xerror.InvalidArgument.Code()
		case errors.Is(err, errItemUseTargetNotFound):
			resultID = xerror.NotFound.Code()
		case errors.Is(err, errItemUseFailedPrecondition):
			resultID = xerror.FailedPrecondition.Code()
		}
		xlog.GLog.Warnf("prepare item use failed aid:%d character:%d item:%d pet:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetItemId(), req.GetTargetPetUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemUseRes_CMD), resultID)
		return
	}
	if err := persistCharacterItemUsePlan(plan, p.accountRecord, character, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist item use failed aid:%d character:%d item:%d pet:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetItemId(), req.GetTargetPetUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_ItemUseRes_CMD), xerror.Internal.Code())
		return
	}
	// 先下发持久化后的权威增量, 再回复道具使用成功, 客户端不会看到部分成功状态.
	if plan.characterChanged {
		p.sendCharacterBaseChangedNotify(gateway, plan.next)
	}
	if len(plan.petChangedUUIDs) > 0 {
		changedPetRecordList := make([]*pb.PetRecord, 0, len(plan.petChangedUUIDs))
		for _, petRecord := range plan.next.GetPetRecordList() {
			if petRecord == nil {
				continue
			}
			for _, petUUID := range plan.petChangedUUIDs {
				if petRecord.GetUuid() == petUUID {
					changedPetRecordList = append(changedPetRecordList, petRecord)
					break
				}
			}
		}
		p.sendCharacterPetChangedNotify(gateway, plan.characterUUID, changedPetRecordList)
	}
	p.sendCharacterItemChangedNotify(gateway, plan.characterUUID, map[uint32]uint64{plan.itemID: newCharacterItemManager(plan.next).Count(plan.itemID)})
	p.sendClientRes(gateway, uint32(pb.MsgID_ItemUseRes_CMD), xerror.Success.Code(), &pb.ItemUseRes{
		CharacterUuid: req.GetCharacterUuid(),
		ItemId:        req.GetItemId(),
		TargetPetUuid: req.GetTargetPetUuid(),
	})
}
