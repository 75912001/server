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
	errPetWarehouseInvalidArgument   = errors.New("invalid pet warehouse argument")
	errPetWarehouseTargetNotFound    = errors.New("pet warehouse target not found")
	errPetWarehouseResourceExhausted = errors.New("pet warehouse capacity exhausted")
	errPetWarehouseRecordInvalid     = errors.New("pet warehouse record is invalid")
)

type petWarehouseTransferPlan struct {
	character      *pb.CharacterRecord
	pet            *pb.PetRecord
	previousPets   []*pb.PetRecord
	previousStatus pb.PetCarryStatus
	deposit        bool
}

func (p *Account) onPetWarehouseDepositReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.PetWarehouseDepositReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), xerror.NotFound.Code())
		return
	}
	if character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := preparePetWarehouseDepositPlan(p.accountRecord, character.record, req.GetPetUuid())
	if err != nil {
		p.logPetWarehouseFailure("deposit", req.GetCharacterUuid(), req.GetPetUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), petWarehouseResultID(err))
		return
	}
	if err := persistPetWarehouseTransfer(plan, p.accountRecord.GetPetWarehouseRecordMap(), func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist pet warehouse deposit failed aid:%d character:%d pet:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetPetUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_PetWarehouseDepositRes_CMD), xerror.Success.Code(), &pb.PetWarehouseDepositRes{
		CharacterUuid: req.GetCharacterUuid(),
		PetUuid:       req.GetPetUuid(),
	})
}

func (p *Account) onPetWarehouseWithdrawReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.PetWarehouseWithdrawReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), xerror.NotFound.Code())
		return
	}
	if character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	plan, err := preparePetWarehouseWithdrawPlan(p.accountRecord, character.record, req.GetPetUuid())
	if err != nil {
		p.logPetWarehouseFailure("withdraw", req.GetCharacterUuid(), req.GetPetUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), petWarehouseResultID(err))
		return
	}
	if err := persistPetWarehouseTransfer(plan, p.accountRecord.GetPetWarehouseRecordMap(), func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist pet warehouse withdraw failed aid:%d character:%d pet:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetPetUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_PetWarehouseWithdrawRes_CMD), xerror.Success.Code(), &pb.PetWarehouseWithdrawRes{
		CharacterUuid: req.GetCharacterUuid(),
		PetUuid:       req.GetPetUuid(),
	})
}

func preparePetWarehouseDepositPlan(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	petUUID uint64,
) (*petWarehouseTransferPlan, error) {
	warehouse, err := validatePetWarehouseTransferRecords(accountRecord, characterRecord, petUUID)
	if err != nil {
		return nil, err
	}
	if len(warehouse) >= int(pb.AccountRecordLimit_AccountRecordLimit_MaxPetWarehouseCount) {
		return nil, fmt.Errorf("%w: warehouse count %d", errPetWarehouseResourceExhausted, len(warehouse))
	}
	if _, exists := warehouse[petUUID]; exists {
		return nil, fmt.Errorf("%w: pet %d already exists in warehouse", errPetWarehouseRecordInvalid, petUUID)
	}

	var targetPet *pb.PetRecord
	seen := make(map[uint64]struct{}, len(characterRecord.GetPetRecordList()))
	for index, petRecord := range characterRecord.GetPetRecordList() {
		if petRecord == nil || petRecord.GetUuid() == 0 {
			return nil, fmt.Errorf("%w: carried pet %d is empty", errPetWarehouseRecordInvalid, index)
		}
		if _, exists := seen[petRecord.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: carried pet %d is duplicated", errPetWarehouseRecordInvalid, petRecord.GetUuid())
		}
		seen[petRecord.GetUuid()] = struct{}{}
		if petRecord.GetUuid() == petUUID {
			targetPet = petRecord
		}
	}
	if targetPet == nil {
		return nil, fmt.Errorf("%w: carried pet %d", errPetWarehouseTargetNotFound, petUUID)
	}

	previousPets := append([]*pb.PetRecord(nil), characterRecord.GetPetRecordList()...)
	return &petWarehouseTransferPlan{
		character:      characterRecord,
		pet:            targetPet,
		previousPets:   previousPets,
		previousStatus: targetPet.GetCarryStatus(),
		deposit:        true,
	}, nil
}

func preparePetWarehouseWithdrawPlan(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	petUUID uint64,
) (*petWarehouseTransferPlan, error) {
	warehouse, err := validatePetWarehouseTransferRecords(accountRecord, characterRecord, petUUID)
	if err != nil {
		return nil, err
	}
	if len(characterRecord.GetPetRecordList()) >= int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return nil, fmt.Errorf(
			"%w: carried pet count %d",
			errPetWarehouseResourceExhausted,
			len(characterRecord.GetPetRecordList()),
		)
	}

	targetPet, exists := warehouse[petUUID]
	if !exists {
		return nil, fmt.Errorf("%w: warehouse pet %d", errPetWarehouseTargetNotFound, petUUID)
	}
	if targetPet == nil || targetPet.GetUuid() != petUUID || targetPet.GetCarryStatus() != pb.PetCarryStatus_PetCarryStatus_Rest {
		return nil, fmt.Errorf("%w: warehouse pet %d is invalid", errPetWarehouseRecordInvalid, petUUID)
	}
	for index, petRecord := range characterRecord.GetPetRecordList() {
		if petRecord == nil || petRecord.GetUuid() == 0 {
			return nil, fmt.Errorf("%w: carried pet %d is empty", errPetWarehouseRecordInvalid, index)
		}
		if petRecord.GetUuid() == petUUID {
			return nil, fmt.Errorf("%w: pet %d also exists in carried list", errPetWarehouseRecordInvalid, petUUID)
		}
	}

	return &petWarehouseTransferPlan{
		character:      characterRecord,
		pet:            targetPet,
		previousPets:   append([]*pb.PetRecord(nil), characterRecord.GetPetRecordList()...),
		previousStatus: targetPet.GetCarryStatus(),
	}, nil
}

func validatePetWarehouseTransferRecords(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	petUUID uint64,
) (map[uint64]*pb.PetRecord, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase() == nil || petUUID == 0 {
		return nil, errPetWarehouseInvalidArgument
	}
	warehouse := accountRecord.GetPetWarehouseRecordMap()
	if warehouse == nil {
		return nil, fmt.Errorf("%w: warehouse map is nil", errPetWarehouseRecordInvalid)
	}
	return warehouse, nil
}

// persistPetWarehouseTransfer 只在计划完成全部本地校验后修改档案, cache 失败时恢复宠物位置和原状态.
func persistPetWarehouseTransfer(
	plan *petWarehouseTransferPlan,
	warehouse map[uint64]*pb.PetRecord,
	persist func() error,
) error {
	if plan == nil || plan.character == nil || plan.pet == nil || warehouse == nil || persist == nil {
		return errPetWarehouseInvalidArgument
	}

	petUUID := plan.pet.GetUuid()
	if plan.deposit {
		nextPets := make([]*pb.PetRecord, 0, len(plan.previousPets)-1)
		for _, petRecord := range plan.previousPets {
			if petRecord != plan.pet {
				nextPets = append(nextPets, petRecord)
			}
		}
		plan.pet.CarryStatus = pb.PetCarryStatus_PetCarryStatus_Rest
		plan.character.PetRecordList = nextPets
		warehouse[petUUID] = plan.pet
	} else {
		plan.character.PetRecordList = append(append([]*pb.PetRecord(nil), plan.previousPets...), plan.pet)
		delete(warehouse, petUUID)
	}

	if err := persist(); err != nil {
		plan.pet.CarryStatus = plan.previousStatus
		plan.character.PetRecordList = plan.previousPets
		if plan.deposit {
			delete(warehouse, petUUID)
		} else {
			warehouse[petUUID] = plan.pet
		}
		return err
	}
	return nil
}

func petWarehouseResultID(err error) uint32 {
	switch {
	case errors.Is(err, errPetWarehouseInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errPetWarehouseTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errPetWarehouseResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}

func (p *Account) logPetWarehouseFailure(operation string, characterUUID uint64, petUUID uint64, err error) {
	xlog.GLog.Warnf(
		"prepare pet warehouse %s failed aid:%d character:%d pet:%d err:%v",
		operation,
		p.aid,
		characterUUID,
		petUUID,
		err,
	)
}
