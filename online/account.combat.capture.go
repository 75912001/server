package main

import (
	"context"
	"fmt"
	"math"

	commonpet "server/common/pet"
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	"google.golang.org/protobuf/proto"
)

// combatRoomCaptureInput 跨 actor 只传递不可变个体快照和房间身份, 不读取 CombatRoom 运行态.
type combatRoomCaptureInput struct {
	characterUUID uint64
	combatRoom    *xactor.Actor[string]
	gateway       *Gateway
	snapshot      *commonpet.CaptureSnapshot
}

type combatRoomCaptureResult struct {
	petUUID uint64
	reason  pb.CombatCaptureFailureReason
	err     error
}

// PostCaptureCombatPetSync 等待账号保存结果, 房间收到成功后才能移除敌人和发布 Get!.
func (p *Account) PostCaptureCombatPetSync(input combatRoomCaptureInput) combatRoomCaptureResult {
	response, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdCapturePet, input))
	if err != nil {
		return combatRoomCaptureResult{reason: pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence, err: err}
	}
	result, ok := response.(combatRoomCaptureResult)
	if !ok {
		return combatRoomCaptureResult{
			reason: pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence,
			err:    fmt.Errorf("capture pet account response is invalid"),
		}
	}
	return result
}

func (p *Account) captureCombatPet(input combatRoomCaptureInput) combatRoomCaptureResult {
	if p.accountRecord == nil || p.characterManager == nil || input.characterUUID == 0 || input.combatRoom == nil || input.snapshot == nil {
		return combatRoomCaptureResult{reason: pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence, err: fmt.Errorf("capture pet account input is invalid")}
	}
	character := p.characterManager.find(input.characterUUID)
	if character == nil || !character.online || character.combatRoom != input.combatRoom {
		return combatRoomCaptureResult{reason: pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence, err: fmt.Errorf("capture pet character is no longer in this battle")}
	}
	petRecord, reason, err := persistCombatCapturedPet(p.accountRecord, character, input.snapshot, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	})
	if err != nil || petRecord == nil {
		return combatRoomCaptureResult{reason: reason, err: err}
	}
	p.sendCharacterPetChangedNotify(input.gateway, input.characterUUID, []*pb.PetRecord{petRecord})
	return combatRoomCaptureResult{petUUID: petRecord.GetUuid()}
}

// persistCombatCapturedPet 在账号 actor 内检查当前容量并提交档案. 保存失败时恢复角色指针和 UUID 游标.
func persistCombatCapturedPet(accountRecord *pb.AccountRecord, character *character, snapshot *commonpet.CaptureSnapshot, persist func() error) (*pb.PetRecord, pb.CombatCaptureFailureReason, error) {
	failed := pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence
	if accountRecord == nil || character == nil || character.record == nil || snapshot == nil || persist == nil {
		return nil, failed, fmt.Errorf("capture pet persistence input is invalid")
	}
	previous := character.record
	if len(previous.GetPetRecordList()) >= int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return nil, pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Capacity, nil
	}
	previousUsedUUID := accountRecord.GetUsedUuid()
	if previousUsedUUID == math.MaxUint64 {
		return nil, failed, fmt.Errorf("capture pet account uuid is exhausted")
	}
	slot := -1
	for index, record := range accountRecord.GetCharacterRecordList() {
		if record == previous && record.GetBase().GetUuid() != 0 {
			slot = index
			break
		}
	}
	if slot < 0 {
		return nil, failed, fmt.Errorf("capture pet character record slot is missing")
	}
	petRecord, err := snapshot.NewRecord(previousUsedUUID + 1)
	if err != nil {
		return nil, failed, err
	}
	next := proto.Clone(previous).(*pb.CharacterRecord)
	next.PetRecordList = append(next.PetRecordList, petRecord)
	accountRecord.UsedUuid = petRecord.GetUuid()
	accountRecord.CharacterRecordList[slot] = next
	character.record = next
	if err := persist(); err != nil {
		accountRecord.UsedUuid = previousUsedUUID
		accountRecord.CharacterRecordList[slot] = previous
		character.record = previous
		return nil, failed, err
	}
	return petRecord, pb.CombatCaptureFailureReason_CombatCaptureFailureReason_None, nil
}
