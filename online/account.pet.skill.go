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

// petSkillSetPlan 保存技能槽与石币已经同时写入的候选账号快照. cache 成功前不修改在线权威档案.
type petSkillSetPlan struct {
	characterUUID     uint64
	petUUID           uint64
	slotIndex         uint32
	skillID           uint32
	cost              uint64
	affectedItem      *pb.ItemElement
	petRecord         *pb.PetRecord
	characterSlot     int
	previousCharacter *pb.CharacterRecord
	nextCharacter     *pb.CharacterRecord
	nextAccountRecord *pb.AccountRecord
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

	if err := persistPetSkillSetPlan(plan, p.accountRecord, character, func(nextAccountRecord *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, nextAccountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist pet skill set failed aid:%d character:%d pet:%d slot:%d skill:%d cost:%d err:%v",
			p.aid,
			plan.characterUUID,
			plan.petUUID,
			plan.slotIndex,
			plan.skillID,
			plan.cost,
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_PetSkillSetRes_CMD), xerror.Success.Code(), &pb.PetSkillSetRes{
		CharacterUuid: plan.characterUUID,
		PetUuid:       plan.petUUID,
		SlotIndex:     plan.slotIndex,
		SkillId:       plan.skillID,
		Cost:          plan.cost,
		AffectedItem:  proto.Clone(plan.affectedItem).(*pb.ItemElement),
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

// preparePetSkillSetPlan 在账号副本中同时完成技能槽写入和石币扣除. skillID为0时只清空槽位且不退款.
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

	cost := uint64(0)
	if skillID != 0 {
		if !assetIDInRange(uint64(skillID), pb.AssetIDRange_AssetIDRange_Skill_Start, pb.AssetIDRange_AssetIDRange_Skill_End) {
			return nil, fmt.Errorf("%w: skill %d is outside skill range", errPetSkillSetInvalidArgument, skillID)
		}
		if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Skill == nil {
			return nil, fmt.Errorf("%w: skill config is not loaded", errPetSkillSetRecordInvalid)
		}
		skillEntry := gameconfig.GGameConfig.Skill.Get(skillID)
		if skillEntry == nil {
			return nil, fmt.Errorf("%w: skill %d", errPetSkillSetTargetNotFound, skillID)
		}
		if skillEntry.ID == nil || *skillEntry.ID != skillID {
			return nil, fmt.Errorf("%w: skill %d config id mismatch", errPetSkillSetRecordInvalid, skillID)
		}
		if skillEntry.Cost == nil {
			return nil, fmt.Errorf("%w: skill %d is not pet-learnable", errPetSkillSetFailedPrecondition, skillID)
		}
		cost = *skillEntry.Cost
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

	stoneID := uint32(pb.AssetID_AssetID_Stone)
	if newCharacterItemManager(characterRecord).Count(stoneID) < cost {
		return nil, fmt.Errorf("%w: stone is less than skill cost %d", errPetSkillSetFailedPrecondition, cost)
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextCharacter := nextAccountRecord.GetCharacterRecordList()[characterSlot]
	nextPetRecord := nextCharacter.GetPetRecordList()[petIndex]
	if cost != 0 {
		if err := newCharacterItemManager(nextCharacter).Consume(stoneID, cost); err != nil {
			return nil, fmt.Errorf("%w: consume stone: %v", errPetSkillSetRecordInvalid, err)
		}
	}
	nextPetRecord.SkillIdList[slotIndex] = skillID
	affectedItem := &pb.ItemElement{AssetId: stoneID, Count: newCharacterItemManager(nextCharacter).Count(stoneID)}

	return &petSkillSetPlan{
		characterUUID:     characterRecord.GetBase().GetUuid(),
		petUUID:           petUUID,
		slotIndex:         slotIndex,
		skillID:           skillID,
		cost:              cost,
		affectedItem:      affectedItem,
		petRecord:         nextPetRecord,
		characterSlot:     characterSlot,
		previousCharacter: characterRecord,
		nextCharacter:     nextCharacter,
		nextAccountRecord: nextAccountRecord,
	}, nil
}

// persistPetSkillSetPlan 先持久化完整候选账号快照, 成功后再一次性替换在线角色档案引用.
func persistPetSkillSetPlan(
	plan *petSkillSetPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func(*pb.AccountRecord) error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.nextAccountRecord == nil || plan.nextCharacter == nil || plan.petRecord == nil || plan.affectedItem == nil {
		return errPetSkillSetInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) || accountRecord.GetCharacterRecordList()[plan.characterSlot] != plan.previousCharacter || character.record != plan.previousCharacter {
		return fmt.Errorf("%w: authoritative account state changed before persistence", errPetSkillSetRecordInvalid)
	}
	if err := persist(plan.nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[plan.characterSlot] = plan.nextCharacter
	character.record = plan.nextCharacter
	return nil
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
