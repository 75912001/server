package main

import (
	"errors"
	"fmt"
	"server/common"
	pb "server/proto/pb"
	"strings"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errPetNickInvalidArgument = errors.New("invalid pet nick argument")
	errPetNickTargetNotFound  = errors.New("pet nick target not found")
	errPetNickRecordInvalid   = errors.New("pet nick record is invalid")
)

type petNickSetPlan struct {
	pet          *pb.PetRecord
	previousNick string
	nextNick     string
}

func (p *Account) onPetNickSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.PetNickSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetPetUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetNickSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_PetNickSetRes_CMD), xerror.NotFound.Code())
		return
	}

	plan, err := preparePetNickSetPlan(character.record, req.GetPetUuid(), req.GetNick())
	if err != nil {
		xlog.GLog.Warnf(
			"prepare pet nick set failed aid:%d character:%d pet:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetPetUuid(),
			err,
		)
		resultID := xerror.Internal.Code()
		switch {
		case errors.Is(err, errPetNickInvalidArgument):
			resultID = xerror.InvalidArgument.Code()
		case errors.Is(err, errPetNickTargetNotFound):
			resultID = xerror.NotFound.Code()
		}
		p.sendClientErr(gateway, uint32(pb.MsgID_PetNickSetRes_CMD), resultID)
		return
	}

	if err := persistPetNickSet(plan, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf(
			"persist pet nick set failed aid:%d character:%d pet:%d err:%v",
			p.aid,
			req.GetCharacterUuid(),
			req.GetPetUuid(),
			err,
		)
		p.sendClientErr(gateway, uint32(pb.MsgID_PetNickSetRes_CMD), xerror.Internal.Code())
		return
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_PetNickSetRes_CMD), xerror.Success.Code(), &pb.PetNickSetRes{
		CharacterUuid: req.GetCharacterUuid(),
		PetUuid:       plan.pet.GetUuid(),
		Nick:          plan.pet.GetNick(),
	})
}

func preparePetNickSetPlan(characterRecord *pb.CharacterRecord, petUUID uint64, requestedNick string) (*petNickSetPlan, error) {
	if characterRecord == nil || characterRecord.GetBase() == nil || petUUID == 0 {
		return nil, errPetNickInvalidArgument
	}
	nick := strings.TrimSpace(requestedNick)
	if nick != "" && !common.IsValidCharacterNick(nick) {
		return nil, fmt.Errorf("%w: nick length is invalid", errPetNickInvalidArgument)
	}

	var targetPet *pb.PetRecord
	seenPetUUID := make(map[uint64]struct{}, len(characterRecord.GetPetRecordList()))
	for index, petRecord := range characterRecord.GetPetRecordList() {
		if petRecord == nil || petRecord.GetUuid() == 0 {
			return nil, fmt.Errorf("%w: carried pet %d is empty", errPetNickRecordInvalid, index)
		}
		if _, exists := seenPetUUID[petRecord.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: carried pet %d is duplicated", errPetNickRecordInvalid, petRecord.GetUuid())
		}
		seenPetUUID[petRecord.GetUuid()] = struct{}{}
		if petRecord.GetUuid() == petUUID {
			targetPet = petRecord
		}
	}
	if targetPet == nil {
		return nil, fmt.Errorf("%w: carried pet %d", errPetNickTargetNotFound, petUUID)
	}

	return &petNickSetPlan{
		pet:          targetPet,
		previousNick: targetPet.GetNick(),
		nextNick:     nick,
	}, nil
}

// persistPetNickSet 只修改目标宠物昵称, cache 写入失败时恢复原值.
func persistPetNickSet(plan *petNickSetPlan, persist func() error) error {
	if plan == nil || plan.pet == nil || persist == nil {
		return errPetNickInvalidArgument
	}
	plan.pet.Nick = plan.nextNick
	if err := persist(); err != nil {
		plan.pet.Nick = plan.previousNick
		return err
	}
	return nil
}
