package main

import (
	"errors"
	"fmt"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errPetCarryStatusInvalidArgument    = errors.New("invalid pet carry status argument")
	errPetCarryStatusTargetNotFound     = errors.New("carried pet not found")
	errPetCarryStatusFailedPrecondition = errors.New("pet carry status precondition failed")
	errPetCarryStatusRecordInvalid      = errors.New("current pet carry status record is invalid")
)

type petCarryStatusChange struct {
	record         *pb.PetRecord
	previousStatus pb.PetCarryStatus
	nextStatus     pb.PetCarryStatus
}

type petCarryStatusChangePlan struct {
	changes []petCarryStatusChange
}

func (p *Account) onPetCarryStatusSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.PetCarryStatusSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	change := req.GetChange()
	if req.GetCharacterUuid() == 0 || change == nil || change.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), xerror.NotFound.Code())
		return
	}

	plan, err := preparePetCarryStatusChangePlan(character.record, change.GetPetUuid(), change.GetCarryStatus())
	if err != nil {
		resultID := xerror.Internal.Code()
		switch {
		case errors.Is(err, errPetCarryStatusInvalidArgument):
			resultID = xerror.InvalidArgument.Code()
		case errors.Is(err, errPetCarryStatusTargetNotFound):
			resultID = xerror.NotFound.Code()
		case errors.Is(err, errPetCarryStatusFailedPrecondition):
			resultID = xerror.FailedPrecondition.Code()
		}
		xlog.GLog.Warnf(
			"prepare pet carry status change failed aid:%d character:%d pet:%d status:%s err:%v",
			p.aid,
			req.GetCharacterUuid(),
			change.GetPetUuid(),
			change.GetCarryStatus(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), resultID)
		return
	}

	if err := persistPetCarryStatusChange(plan, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"set account record after pet carry status change failed aid:%d character:%d pet:%d status:%s err:%v",
			p.aid,
			req.GetCharacterUuid(),
			change.GetPetUuid(),
			change.GetCarryStatus(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), xerror.Internal.Code())
		return
	}
	changedPetRecordList := make([]*pb.PetRecord, 0, len(plan.changes))
	for _, change := range plan.changes {
		changedPetRecordList = append(changedPetRecordList, change.record)
	}
	p.sendCharacterPetChangedNotify(gateway, character.record.GetBase().GetUuid(), changedPetRecordList)

	p.sendClientRes(gateway, uint32(pb.MsgID_PetCarryStatusSetRes_CMD), xerror.Success.Code(), &pb.PetCarryStatusSetRes{
		CharacterUuid: req.GetCharacterUuid(),
		ChangeList:    plan.responseChangeList(),
	})
}

// preparePetCarryStatusChangePlan 在产生任何状态变更前完成请求和当前档案校验.
func preparePetCarryStatusChangePlan(
	characterRecord *pb.CharacterRecord,
	targetPetUUID uint64,
	targetStatus pb.PetCarryStatus,
) (*petCarryStatusChangePlan, error) {
	if characterRecord == nil || targetPetUUID == 0 {
		return nil, fmt.Errorf("%w: character or pet uuid is empty", errPetCarryStatusInvalidArgument)
	}
	if !isPetCarryStatusValid(targetStatus) {
		return nil, fmt.Errorf("%w: target status %s", errPetCarryStatusInvalidArgument, targetStatus)
	}

	petRecordList := characterRecord.GetPetRecordList()
	var targetPet *pb.PetRecord
	battleCount := 0
	mountCount := 0
	seenPetUUID := make(map[uint64]struct{}, len(petRecordList))
	for index, petRecord := range petRecordList {
		if petRecord == nil || petRecord.GetUuid() == 0 {
			return nil, fmt.Errorf("%w: carried pet %d is empty", errPetCarryStatusRecordInvalid, index)
		}
		if _, exists := seenPetUUID[petRecord.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: pet uuid %d is duplicated", errPetCarryStatusRecordInvalid, petRecord.GetUuid())
		}
		seenPetUUID[petRecord.GetUuid()] = struct{}{}
		if !isPetCarryStatusValid(petRecord.GetCarryStatus()) {
			return nil, fmt.Errorf(
				"%w: pet %d status %s",
				errPetCarryStatusRecordInvalid,
				petRecord.GetUuid(),
				petRecord.GetCarryStatus(),
			)
		}
		switch petRecord.GetCarryStatus() {
		case pb.PetCarryStatus_PetCarryStatus_Battle:
			battleCount++
		case pb.PetCarryStatus_PetCarryStatus_Mount:
			mountCount++
		}
		if petRecord.GetUuid() == targetPetUUID {
			targetPet = petRecord
		}
	}
	if battleCount > 1 || mountCount > 1 {
		return nil, fmt.Errorf(
			"%w: battle count %d, mount count %d",
			errPetCarryStatusRecordInvalid,
			battleCount,
			mountCount,
		)
	}
	if targetPet == nil {
		return nil, fmt.Errorf("%w: pet uuid %d", errPetCarryStatusTargetNotFound, targetPetUUID)
	}
	if targetStatus == pb.PetCarryStatus_PetCarryStatus_Mount && targetPet.GetLoyalty() != 100 {
		return nil, fmt.Errorf(
			"%w: mount pet %d loyalty %d is not 100",
			errPetCarryStatusFailedPrecondition,
			targetPetUUID,
			targetPet.GetLoyalty(),
		)
	}

	plan := &petCarryStatusChangePlan{
		changes: make([]petCarryStatusChange, 0, 2),
	}
	for _, petRecord := range petRecordList {
		nextStatus := petRecord.GetCarryStatus()
		switch {
		case petRecord == targetPet:
			nextStatus = targetStatus
		case targetStatus == pb.PetCarryStatus_PetCarryStatus_Battle &&
			petRecord.GetCarryStatus() == pb.PetCarryStatus_PetCarryStatus_Battle:
			nextStatus = pb.PetCarryStatus_PetCarryStatus_Wait
		case targetStatus == pb.PetCarryStatus_PetCarryStatus_Mount &&
			petRecord.GetCarryStatus() == pb.PetCarryStatus_PetCarryStatus_Mount:
			nextStatus = pb.PetCarryStatus_PetCarryStatus_Wait
		}
		if nextStatus == petRecord.GetCarryStatus() {
			continue
		}
		plan.changes = append(plan.changes, petCarryStatusChange{
			record:         petRecord,
			previousStatus: petRecord.GetCarryStatus(),
			nextStatus:     nextStatus,
		})
	}
	return plan, nil
}

func isPetCarryStatusValid(status pb.PetCarryStatus) bool {
	switch status {
	case pb.PetCarryStatus_PetCarryStatus_Rest,
		pb.PetCarryStatus_PetCarryStatus_Battle,
		pb.PetCarryStatus_PetCarryStatus_Wait,
		pb.PetCarryStatus_PetCarryStatus_Mount:
		return true
	default:
		return false
	}
}

// persistPetCarryStatusChange 将同一计划中的状态一起写入账号档案, 持久化失败时恢复全部旧状态.
func persistPetCarryStatusChange(plan *petCarryStatusChangePlan, persist func() error) error {
	if plan == nil {
		return fmt.Errorf("pet carry status change plan is nil")
	}
	if len(plan.changes) == 0 {
		return nil
	}
	for _, change := range plan.changes {
		change.record.CarryStatus = change.nextStatus
	}
	if err := persist(); err != nil {
		for _, change := range plan.changes {
			change.record.CarryStatus = change.previousStatus
		}
		return err
	}
	return nil
}

func (p *petCarryStatusChangePlan) responseChangeList() []*pb.PetCarryStatusChange {
	changeList := make([]*pb.PetCarryStatusChange, 0, len(p.changes))
	for _, change := range p.changes {
		changeList = append(changeList, &pb.PetCarryStatusChange{
			PetUuid:     change.record.GetUuid(),
			CarryStatus: change.nextStatus,
		})
	}
	return changeList
}
