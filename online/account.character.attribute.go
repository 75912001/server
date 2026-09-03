package main

import (
	"errors"
	"fmt"
	"math"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errCharacterAttributeAddInvalidArgument      = errors.New("invalid character attribute add argument")
	errCharacterAttributeAddFailedPrecondition   = errors.New("character attribute add precondition failed")
	errCharacterAttributeAddRecordInvalid        = errors.New("character attribute add record is invalid")
	errCharacterAttributeResetInvalidArgument    = errors.New("invalid character attribute reset argument")
	errCharacterAttributeResetFailedPrecondition = errors.New("character attribute reset precondition failed")
	errCharacterAttributeResetRecordInvalid      = errors.New("character attribute reset record is invalid")
)

const characterAttributeResetMaxTotalPoint uint64 = 1000

type characterAttributeAddPlan struct {
	characterUUID uint64
	attributeType pb.CharacterAttributeType
	previous      *pb.CharacterRecord
	next          *pb.CharacterRecord
}

type characterAttributeResetPlan struct {
	characterUUID uint64
	previous      *pb.CharacterRecord
	next          *pb.CharacterRecord
}

func (p *Account) onCharacterAttributeAddReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterAttributeAddReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := prepareCharacterAttributeAddPlan(character.record, req.GetAttributeType())
	if err != nil {
		resultID := xerror.Internal.Code()
		switch {
		case errors.Is(err, errCharacterAttributeAddInvalidArgument):
			resultID = xerror.InvalidArgument.Code()
		case errors.Is(err, errCharacterAttributeAddFailedPrecondition):
			resultID = xerror.FailedPrecondition.Code()
		}
		xlog.GLog.Warnf(
			"prepare character attribute add failed aid:%d character:%d attribute:%s err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetAttributeType(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), resultID)
		return
	}

	if err := persistCharacterAttributeAddPlan(plan, p.accountRecord, character, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist character attribute add failed aid:%d character:%d attribute:%s err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetAttributeType(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), xerror.Internal.Code())
		return
	}

	// 先下发持久化后的角色基础权威快照, 再解除客户端单请求等待状态.
	p.sendCharacterBaseChangedNotify(gateway, plan.next)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterAttributeAddRes_CMD), xerror.Success.Code(), &pb.CharacterAttributeAddRes{
		CharacterUuid: req.GetCharacterUuid(),
		AttributeType: plan.attributeType,
	})
}

func (p *Account) onCharacterAttributeResetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterAttributeResetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetCharacterAttribute() == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := prepareCharacterAttributeResetPlan(character.record, req.GetCharacterAttribute())
	if err != nil {
		resultID := xerror.Internal.Code()
		switch {
		case errors.Is(err, errCharacterAttributeResetInvalidArgument):
			resultID = xerror.InvalidArgument.Code()
		case errors.Is(err, errCharacterAttributeResetFailedPrecondition):
			resultID = xerror.FailedPrecondition.Code()
		}
		xlog.GLog.Warnf(
			"prepare character attribute reset failed aid:%d character:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), resultID)
		return
	}

	if err := persistCharacterAttributeResetPlan(plan, p.accountRecord, character, func(nextAccountRecord *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, nextAccountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist character attribute reset failed aid:%d character:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendCharacterBaseChangedNotify(gateway, plan.next)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.Success.Code(), &pb.CharacterAttributeResetRes{
		CharacterUuid: req.GetCharacterUuid(),
	})
}

// prepareCharacterAttributeAddPlan 只在副本上计算加点结果, 不提前修改 actor 或账号档案.
func prepareCharacterAttributeAddPlan(record *pb.CharacterRecord, attributeType pb.CharacterAttributeType) (*characterAttributeAddPlan, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 {
		return nil, errCharacterAttributeAddInvalidArgument
	}
	if record.GetBase().GetAvailablePoint() == 0 {
		return nil, fmt.Errorf("%w: no available point", errCharacterAttributeAddFailedPrecondition)
	}

	next := proto.Clone(record).(*pb.CharacterRecord)
	base := next.GetBase()
	switch attributeType {
	case pb.CharacterAttributeType_CharacterAttributeType_Vitality:
		if base.GetVitality() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: vitality overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		base.Vitality++
	case pb.CharacterAttributeType_CharacterAttributeType_Strength:
		if base.GetStrength() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: strength overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		base.Strength++
	case pb.CharacterAttributeType_CharacterAttributeType_Toughness:
		if base.GetToughness() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: toughness overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		base.Toughness++
	case pb.CharacterAttributeType_CharacterAttributeType_Dexterity:
		if base.GetDexterity() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: dexterity overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		base.Dexterity++
	default:
		return nil, fmt.Errorf("%w: attribute %d", errCharacterAttributeAddInvalidArgument, attributeType)
	}
	base.AvailablePoint--
	return &characterAttributeAddPlan{
		characterUUID: record.GetBase().GetUuid(),
		attributeType: attributeType,
		previous:      record,
		next:          next,
	}, nil
}

// prepareCharacterAttributeResetPlan 根据权威总点数校验客户端提交的四项最终属性, 并在档案副本上计算剩余可加点.
func prepareCharacterAttributeResetPlan(record *pb.CharacterRecord, target *pb.CharacterAttributePoints) (*characterAttributeResetPlan, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 || target == nil {
		return nil, errCharacterAttributeResetInvalidArgument
	}

	base := record.GetBase()
	currentTotalPoint := uint64(base.GetVitality()) + uint64(base.GetStrength()) + uint64(base.GetToughness()) + uint64(base.GetDexterity()) + uint64(base.GetAvailablePoint())
	if currentTotalPoint < uint64(pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint) || currentTotalPoint > characterAttributeResetMaxTotalPoint {
		return nil, fmt.Errorf("%w: current total point %d is outside [%d,%d]", errCharacterAttributeResetFailedPrecondition, currentTotalPoint, pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint, characterAttributeResetMaxTotalPoint)
	}

	targetAllocatedPoint := uint64(target.GetVitality()) + uint64(target.GetStrength()) + uint64(target.GetToughness()) + uint64(target.GetDexterity())
	if targetAllocatedPoint < uint64(pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint) {
		return nil, fmt.Errorf("%w: target allocated point %d is below %d", errCharacterAttributeResetFailedPrecondition, targetAllocatedPoint, pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint)
	}
	if targetAllocatedPoint > currentTotalPoint {
		return nil, fmt.Errorf("%w: target allocated point %d exceeds current total %d", errCharacterAttributeResetFailedPrecondition, targetAllocatedPoint, currentTotalPoint)
	}
	availablePoint := uint32(currentTotalPoint - targetAllocatedPoint)

	next := proto.Clone(record).(*pb.CharacterRecord)
	nextBase := next.GetBase()
	nextBase.Vitality = target.GetVitality()
	nextBase.Strength = target.GetStrength()
	nextBase.Toughness = target.GetToughness()
	nextBase.Dexterity = target.GetDexterity()
	nextBase.AvailablePoint = availablePoint
	return &characterAttributeResetPlan{
		characterUUID: base.GetUuid(),
		previous:      record,
		next:          next,
	}, nil
}

// persistCharacterAttributeAddPlan 在 cache 成功前只临时替换账号槽位, 失败时恢复原记录.
func persistCharacterAttributeAddPlan(
	plan *characterAttributeAddPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func() error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil {
		return errCharacterAttributeAddInvalidArgument
	}
	slot := -1
	for index, record := range accountRecord.GetCharacterRecordList() {
		if record == plan.previous && record.GetBase().GetUuid() == plan.characterUUID {
			slot = index
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("%w: character %d record slot not found", errCharacterAttributeAddRecordInvalid, plan.characterUUID)
	}

	accountRecord.CharacterRecordList[slot] = plan.next
	if err := persist(); err != nil {
		accountRecord.CharacterRecordList[slot] = plan.previous
		return err
	}
	character.record = plan.next
	return nil
}

// persistCharacterAttributeResetPlan 使用独立账号副本写 cache, 成功前不修改 online 权威档案.
func persistCharacterAttributeResetPlan(
	plan *characterAttributeResetPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func(*pb.AccountRecord) error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil {
		return errCharacterAttributeResetInvalidArgument
	}
	slot := -1
	for index, record := range accountRecord.GetCharacterRecordList() {
		if record == plan.previous && record.GetBase().GetUuid() == plan.characterUUID {
			slot = index
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("%w: character %d record slot not found", errCharacterAttributeResetRecordInvalid, plan.characterUUID)
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextAccountRecord.CharacterRecordList[slot] = plan.next
	if err := persist(nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[slot] = plan.next
	character.record = plan.next
	return nil
}
