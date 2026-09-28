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

// characterAttributeAddPlan 描述一次加点. 校验阶段不修改权威档案.
type characterAttributeAddPlan struct {
	characterUUID uint64
	attributeType pb.CharacterAttributeType
}

// characterAttributeResetPlan 描述一次洗点. 校验阶段不修改权威档案.
type characterAttributeResetPlan struct {
	characterUUID uint64
	target        *pb.CharacterAttributePoints
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

	if err := applyCharacterAttributeAddPlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
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
	p.sendCharacterBaseChangedNotify(gateway, character.record)
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

	if err := applyCharacterAttributeResetPlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf(
			"persist character attribute reset failed aid:%d character:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendCharacterBaseChangedNotify(gateway, character.record)
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterAttributeResetRes_CMD), xerror.Success.Code(), &pb.CharacterAttributeResetRes{
		CharacterUuid: req.GetCharacterUuid(),
	})
}

// prepareCharacterAttributeAddPlan 只校验并计算加点目标, 不提前修改 actor 或账号档案.
func prepareCharacterAttributeAddPlan(record *pb.CharacterRecord, attributeType pb.CharacterAttributeType) (*characterAttributeAddPlan, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 {
		return nil, errCharacterAttributeAddInvalidArgument
	}
	if record.GetBase().GetAvailablePoint() == 0 {
		return nil, fmt.Errorf("%w: no available point", errCharacterAttributeAddFailedPrecondition)
	}

	attribute := record.GetBase().GetAttribute()
	switch attributeType {
	case pb.CharacterAttributeType_CharacterAttributeType_Vitality:
		if attribute.GetVitality() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: vitality overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
	case pb.CharacterAttributeType_CharacterAttributeType_Strength:
		if attribute.GetStrength() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: strength overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
	case pb.CharacterAttributeType_CharacterAttributeType_Toughness:
		if attribute.GetToughness() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: toughness overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
	case pb.CharacterAttributeType_CharacterAttributeType_Dexterity:
		if attribute.GetDexterity() == math.MaxUint32 {
			return nil, fmt.Errorf("%w: dexterity overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
	default:
		return nil, fmt.Errorf("%w: attribute %d", errCharacterAttributeAddInvalidArgument, attributeType)
	}
	return &characterAttributeAddPlan{
		characterUUID: record.GetBase().GetUuid(),
		attributeType: attributeType,
	}, nil
}

// applyCharacterAttributeAddPlan 把加点原地写入权威角色档案, 再通知落盘.
func applyCharacterAttributeAddPlan(plan *characterAttributeAddPlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errCharacterAttributeAddInvalidArgument
	}
	if !accountRecordHasCharacterRecord(accountRecord, character.record, plan.characterUUID) {
		return fmt.Errorf("%w: character %d record slot not found", errCharacterAttributeAddRecordInvalid, plan.characterUUID)
	}

	base := character.record.GetBase()
	if base.GetAvailablePoint() == 0 {
		return fmt.Errorf("%w: no available point", errCharacterAttributeAddFailedPrecondition)
	}
	attribute := base.GetAttribute()
	if attribute == nil {
		attribute = &pb.CharacterAttributePoints{}
		base.Attribute = attribute
	}
	switch plan.attributeType {
	case pb.CharacterAttributeType_CharacterAttributeType_Vitality:
		if attribute.GetVitality() == math.MaxUint32 {
			return fmt.Errorf("%w: vitality overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		attribute.Vitality++
	case pb.CharacterAttributeType_CharacterAttributeType_Strength:
		if attribute.GetStrength() == math.MaxUint32 {
			return fmt.Errorf("%w: strength overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		attribute.Strength++
	case pb.CharacterAttributeType_CharacterAttributeType_Toughness:
		if attribute.GetToughness() == math.MaxUint32 {
			return fmt.Errorf("%w: toughness overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		attribute.Toughness++
	case pb.CharacterAttributeType_CharacterAttributeType_Dexterity:
		if attribute.GetDexterity() == math.MaxUint32 {
			return fmt.Errorf("%w: dexterity overflows uint32", errCharacterAttributeAddFailedPrecondition)
		}
		attribute.Dexterity++
	default:
		return fmt.Errorf("%w: attribute %d", errCharacterAttributeAddInvalidArgument, plan.attributeType)
	}
	base.AvailablePoint--
	return persist()
}

// accountRecordHasCharacterRecord 判断角色记录是否仍是账号档案中该 UUID 的槽位记录.
func accountRecordHasCharacterRecord(accountRecord *pb.AccountRecord, record *pb.CharacterRecord, characterUUID uint64) bool {
	if accountRecord == nil || record == nil {
		return false
	}
	for _, candidate := range accountRecord.GetCharacterRecordList() {
		if candidate == record && candidate.GetBase().GetUuid() == characterUUID {
			return true
		}
	}
	return false
}

// prepareCharacterAttributeResetPlan 根据权威总点数校验客户端提交的四项最终属性, 只计算不修改档案.
func prepareCharacterAttributeResetPlan(record *pb.CharacterRecord, target *pb.CharacterAttributePoints) (*characterAttributeResetPlan, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 || target == nil {
		return nil, errCharacterAttributeResetInvalidArgument
	}

	base := record.GetBase()
	attribute := base.GetAttribute()
	currentTotalPoint := uint64(attribute.GetVitality()) + uint64(attribute.GetStrength()) + uint64(attribute.GetToughness()) + uint64(attribute.GetDexterity()) + uint64(base.GetAvailablePoint())
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

	return &characterAttributeResetPlan{
		characterUUID: base.GetUuid(),
		target:        target,
	}, nil
}

// applyCharacterAttributeResetPlan 把洗点结果原地写入权威角色档案, 再通知落盘.
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知.
func applyCharacterAttributeResetPlan(plan *characterAttributeResetPlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil || plan.target == nil {
		return errCharacterAttributeResetInvalidArgument
	}
	if !accountRecordHasCharacterRecord(accountRecord, character.record, plan.characterUUID) {
		return fmt.Errorf("%w: character %d record slot not found", errCharacterAttributeResetRecordInvalid, plan.characterUUID)
	}

	base := character.record.GetBase()
	attribute := base.GetAttribute()
	currentTotalPoint := uint64(attribute.GetVitality()) + uint64(attribute.GetStrength()) + uint64(attribute.GetToughness()) + uint64(attribute.GetDexterity()) + uint64(base.GetAvailablePoint())
	if currentTotalPoint < uint64(pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint) || currentTotalPoint > characterAttributeResetMaxTotalPoint {
		return fmt.Errorf("%w: current total point %d is outside [%d,%d]", errCharacterAttributeResetFailedPrecondition, currentTotalPoint, pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint, characterAttributeResetMaxTotalPoint)
	}
	targetAllocatedPoint := uint64(plan.target.GetVitality()) + uint64(plan.target.GetStrength()) + uint64(plan.target.GetToughness()) + uint64(plan.target.GetDexterity())
	if targetAllocatedPoint < uint64(pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint) || targetAllocatedPoint > currentTotalPoint {
		return fmt.Errorf("%w: target allocated point %d is outside [%d,%d]", errCharacterAttributeResetFailedPrecondition, targetAllocatedPoint, pb.CharacterLimit_CharacterLimit_CreateAttributeTotalPoint, currentTotalPoint)
	}

	if attribute == nil {
		attribute = &pb.CharacterAttributePoints{}
		base.Attribute = attribute
	}
	attribute.Vitality = plan.target.GetVitality()
	attribute.Strength = plan.target.GetStrength()
	attribute.Toughness = plan.target.GetToughness()
	attribute.Dexterity = plan.target.GetDexterity()
	base.AvailablePoint = uint32(currentTotalPoint - targetAllocatedPoint)
	return persist()
}
