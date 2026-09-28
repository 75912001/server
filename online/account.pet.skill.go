package main

import (
	"errors"
	"fmt"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errPetSkillSetInvalidArgument    = errors.New("invalid pet skill set argument")
	errPetSkillSetTargetNotFound     = errors.New("pet skill set target not found")
	errPetSkillSetFailedPrecondition = errors.New("pet skill set precondition failed")
	errPetSkillSetRecordInvalid      = errors.New("pet skill set record is invalid")
)

// petSkillSetPlan 保存技能槽写入与资源扣除的变更计划. 校验阶段不修改在线权威档案.
type petSkillSetPlan struct {
	characterUUID uint64
	petUUID       uint64
	slotIndex     uint32
	skillID       uint32
	costAmounts   []storeCostAmount
	petIndex      int
	characterSlot int
	// costResultList 在 apply 阶段按消耗后的权威数量填充, 供响应返回.
	costResultList []*pb.ItemCostResult
}

func (p *Account) onPetSkillSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.PetSkillSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if err := validatePetSkillSetCharacterState(character); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), petSkillSetResultID(err))
		return
	}

	plan, err := preparePetSkillSetPlan(
		p.accountRecord,
		character.record,
		req.GetPetUuid(),
		req.GetSlotIndex(),
		req.GetSkillId(),
	)
	if err != nil {
		xlog.GLog.Warnf(
			"pet skill set rejected aid:%d character:%d pet:%d slot:%d skill:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetPetUuid(),
			req.GetSlotIndex(),
			req.GetSkillId(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), petSkillSetResultID(err))
		return
	}

	if err := applyPetSkillSetPlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf(
			"persist pet skill set failed aid:%d character:%d pet:%d slot:%d skill:%d costKinds:%d err:%v",
			p.aid,
			plan.characterUUID,
			plan.petUUID,
			plan.slotIndex,
			plan.skillID,
			len(plan.costResultList),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), xerror.Success.Code(), &pb.PetSkillSetRes{
		CharacterUuid:  plan.characterUUID,
		PetUuid:        plan.petUUID,
		SlotIndex:      plan.slotIndex,
		SkillId:        plan.skillID,
		CostResultList: cloneItemCostResults(plan.costResultList),
	})
}

// validatePetSkillSetCharacterState 约束技能管理只能由已上线且不在战斗中的角色执行.
func validatePetSkillSetCharacterState(character *character) error {
	if character == nil || character.record == nil {
		return errPetSkillSetTargetNotFound
	}
	if !character.online || character.combatRoom != nil {
		return errPetSkillSetFailedPrecondition
	}
	return nil
}

// preparePetSkillSetPlan 在账号副本中同时完成技能槽写入和多资源扣除. skillID为0时只清空槽位且不退款.
func preparePetSkillSetPlan(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	petUUID uint64,
	slotIndex uint32,
	skillID uint32,
) (*petSkillSetPlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || petUUID == 0 || slotIndex >= uint32(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
		return nil, errPetSkillSetInvalidArgument
	}

	var costAmounts []storeCostAmount
	if skillID != 0 {
		if !assetIDInRange(uint64(skillID), pb.AssetID_AssetIDRange_Skill_Start, pb.AssetID_AssetIDRange_Skill_End) {
			return nil, fmt.Errorf("%w: skill %d is outside skill range", errPetSkillSetInvalidArgument, skillID)
		}
		if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Skill == nil || gameconfig.GGameConfig.Store == nil {
			return nil, fmt.Errorf("%w: skill or store config is not loaded", errPetSkillSetRecordInvalid)
		}
		skillEntry := gameconfig.GGameConfig.Skill.Get(skillID)
		if skillEntry == nil {
			return nil, fmt.Errorf("%w: skill %d", errPetSkillSetTargetNotFound, skillID)
		}
		if skillEntry.ID == nil || *skillEntry.ID != skillID {
			return nil, fmt.Errorf("%w: skill %d config id mismatch", errPetSkillSetRecordInvalid, skillID)
		}
		if !skillEntry.CanBeUsedBy("pet") {
			return nil, fmt.Errorf("%w: skill %d is not usable by pets", errPetSkillSetFailedPrecondition, skillID)
		}
		storeEntry, exists := gameconfig.GGameConfig.Store.GetSkillEntryByID(skillID)
		if !exists || storeEntry == nil {
			return nil, fmt.Errorf("%w: skill %d is not sold", errPetSkillSetFailedPrecondition, skillID)
		}
		var err error
		costAmounts, err = prepareStoreCostAmounts(storeEntry, 1, characterRecord)
		if err != nil {
			if errors.Is(err, errStoreCostInsufficient) {
				return nil, fmt.Errorf("%w: %v", errPetSkillSetFailedPrecondition, err)
			}
			return nil, fmt.Errorf("%w: %v", errPetSkillSetRecordInvalid, err)
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
		return nil, fmt.Errorf("%w: character %d record slot not found", errPetSkillSetRecordInvalid, characterRecord.GetBase().GetUuid())
	}

	petIndex := -1
	seenPetUUID := make(map[uint64]struct{}, len(characterRecord.GetPetRecordList()))
	for index, petRecord := range characterRecord.GetPetRecordList() {
		if petRecord == nil || petRecord.GetUuid() == 0 {
			return nil, fmt.Errorf("%w: carried pet %d is empty", errPetSkillSetRecordInvalid, index)
		}
		if _, exists := seenPetUUID[petRecord.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: carried pet %d is duplicated", errPetSkillSetRecordInvalid, petRecord.GetUuid())
		}
		seenPetUUID[petRecord.GetUuid()] = struct{}{}
		if petRecord.GetUuid() == petUUID {
			petIndex = index
		}
	}
	if petIndex < 0 {
		return nil, fmt.Errorf("%w: carried pet %d", errPetSkillSetTargetNotFound, petUUID)
	}
	if len(characterRecord.GetPetRecordList()[petIndex].GetSkillIdList()) != int(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
		return nil, fmt.Errorf("%w: pet %d skill slot count %d", errPetSkillSetRecordInvalid, petUUID, len(characterRecord.GetPetRecordList()[petIndex].GetSkillIdList()))
	}
	return &petSkillSetPlan{
		characterUUID: characterRecord.GetBase().GetUuid(),
		petUUID:       petUUID,
		slotIndex:     slotIndex,
		skillID:       skillID,
		costAmounts:   costAmounts,
		petIndex:      petIndex,
		characterSlot: characterSlot,
	}, nil
}

// applyPetSkillSetPlan 把计划原地应用到权威账号档案, 再通知落盘。
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知。
func applyPetSkillSetPlan(plan *petSkillSetPlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errPetSkillSetInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != character.record {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errPetSkillSetRecordInvalid)
	}
	petRecordList := character.record.GetPetRecordList()
	if plan.petIndex < 0 || plan.petIndex >= len(petRecordList) ||
		petRecordList[plan.petIndex] == nil || petRecordList[plan.petIndex].GetUuid() != plan.petUUID {
		return fmt.Errorf("%w: carried pet %d changed before apply", errPetSkillSetRecordInvalid, plan.petUUID)
	}
	if len(petRecordList[plan.petIndex].GetSkillIdList()) != int(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
		return fmt.Errorf("%w: pet %d skill slot count %d", errPetSkillSetRecordInvalid, plan.petUUID, len(petRecordList[plan.petIndex].GetSkillIdList()))
	}

	costResultList, err := consumeStoreCostAmounts(character.record, plan.costAmounts)
	if err != nil {
		return fmt.Errorf("%w: %v", errPetSkillSetRecordInvalid, err)
	}
	plan.costResultList = costResultList
	petRecordList[plan.petIndex].SkillIdList[plan.slotIndex] = plan.skillID
	return persist()
}

func petSkillSetResultID(err error) uint32 {
	switch {
	case errors.Is(err, errPetSkillSetInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errPetSkillSetTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errPetSkillSetFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	default:
		return xerror.Internal.Code()
	}
}
