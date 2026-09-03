package main

import (
	"errors"
	"fmt"
	"math"
	"time"

	"server/common"
	"server/common/gameconfig"
	commonpet "server/common/pet"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errGMPetAddInvalidArgument    = errors.New("invalid gm pet add argument")
	errGMPetAddTargetNotFound     = errors.New("gm pet add target not found")
	errGMPetAddFailedPrecondition = errors.New("gm pet add precondition failed")
	errGMPetAddResourceExhausted  = errors.New("gm pet add resource exhausted")
)

type gmItemAddPlan struct {
	characterUUID        uint64
	itemID               uint32
	addedCount           uint64
	currentCount         uint64
	previous             *pb.CharacterRecord
	next                 *pb.CharacterRecord
	changedTaskRecordMap map[uint32]*pb.CharacterTaskRecord
}

type gmPetAddPlan struct {
	characterUUID    uint64
	petUUID          uint64
	petID            uint32
	petGrade         pb.PetGrade
	previousUsedUUID uint64
	nextUsedUUID     uint64
	previous         *pb.CharacterRecord
	next             *pb.CharacterRecord
	petRecord        *pb.PetRecord
}

func prepareGMItemAddPlan(record *pb.CharacterRecord, command *pb.GMItemAddCommand) (*gmItemAddPlan, error) {
	if record == nil || record.GetBase().GetUuid() == 0 || command == nil || command.GetItemId() == 0 || command.GetCount() == 0 {
		return nil, fmt.Errorf("%w: character, item id or count is empty", errItemUseInvalidArgument)
	}
	next := proto.Clone(record).(*pb.CharacterRecord)
	if err := newCharacterItemManager(next).Add(command.GetItemId(), command.GetCount()); err != nil {
		return nil, err
	}
	changedTasks, err := newCharacterTaskManager(next).Refresh(time.Now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("advance task after GM item add: %w", err)
	}
	return &gmItemAddPlan{
		characterUUID:        record.GetBase().GetUuid(),
		itemID:               command.GetItemId(),
		addedCount:           command.GetCount(),
		currentCount:         newCharacterItemManager(next).Count(command.GetItemId()),
		previous:             record,
		next:                 next,
		changedTaskRecordMap: changedTasks,
	}, nil
}

func prepareGMPetAddPlan(accountRecord *pb.AccountRecord, record *pb.CharacterRecord, command *pb.GMPetAddCommand) (*gmPetAddPlan, error) {
	if accountRecord == nil || record == nil || record.GetBase().GetUuid() == 0 || command == nil {
		return nil, fmt.Errorf("%w: account, character or command is empty", errGMPetAddInvalidArgument)
	}
	petID := command.GetPetId()
	petGrade := command.GetPetGrade()
	if !assetIDInRange(uint64(petID), pb.AssetIDRange_AssetIDRange_Pet_Start, pb.AssetIDRange_AssetIDRange_Pet_End) ||
		petGrade <= pb.PetGrade_PetGrade_Unknow || petGrade >= pb.PetGrade_PetGrade_Max {
		return nil, fmt.Errorf("%w: pet id %d or grade %s is invalid", errGMPetAddInvalidArgument, petID, petGrade)
	}
	if len(record.GetPetRecordList()) >= int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return nil, fmt.Errorf("%w: carried pet count %d", errGMPetAddResourceExhausted, len(record.GetPetRecordList()))
	}
	if accountRecord.GetUsedUuid() == math.MaxUint64 {
		return nil, fmt.Errorf("%w: account uuid is exhausted", errGMPetAddFailedPrecondition)
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Pet == nil {
		return nil, fmt.Errorf("pet config is not loaded")
	}
	petEntry := gameconfig.GGameConfig.Pet.Get(petID)
	if petEntry == nil {
		return nil, fmt.Errorf("%w: pet config %d not found", errGMPetAddTargetNotFound, petID)
	}

	next := proto.Clone(record).(*pb.CharacterRecord)
	petUUID := accountRecord.GetUsedUuid() + 1
	petRecord, err := commonpet.NewRecord(petEntry, petUUID, 1, petGrade)
	if err != nil {
		return nil, fmt.Errorf("%w: create pet %d: %v", errGMPetAddFailedPrecondition, petID, err)
	}
	petRecord.CarryStatus = pb.PetCarryStatus_PetCarryStatus_Wait
	next.PetRecordList = append(next.PetRecordList, petRecord)
	return &gmPetAddPlan{
		characterUUID:    record.GetBase().GetUuid(),
		petUUID:          petUUID,
		petID:            petID,
		petGrade:         petGrade,
		previousUsedUUID: accountRecord.GetUsedUuid(),
		nextUsedUUID:     petUUID,
		previous:         record,
		next:             next,
		petRecord:        petRecord,
	}, nil
}

func persistGMItemAddPlan(
	plan *gmItemAddPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func() error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil {
		return fmt.Errorf("gm item add persistence input is nil")
	}
	if character.record != plan.previous {
		return fmt.Errorf("character %d authoritative record changed", plan.characterUUID)
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
	character.record = plan.next
	if err := persist(); err != nil {
		accountRecord.CharacterRecordList[slot] = plan.previous
		character.record = plan.previous
		return err
	}
	return nil
}

func persistGMPetAddPlan(
	plan *gmPetAddPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func() error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil {
		return fmt.Errorf("gm pet add persistence input is nil")
	}
	if character.record != plan.previous || accountRecord.GetUsedUuid() != plan.previousUsedUUID {
		return fmt.Errorf("character %d authoritative record changed", plan.characterUUID)
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
	accountRecord.UsedUuid = plan.nextUsedUUID
	accountRecord.CharacterRecordList[slot] = plan.next
	character.record = plan.next
	if err := persist(); err != nil {
		accountRecord.UsedUuid = plan.previousUsedUUID
		accountRecord.CharacterRecordList[slot] = plan.previous
		character.record = plan.previous
		return err
	}
	return nil
}

func gmCommandResultID(err error) uint32 {
	switch {
	case errors.Is(err, errItemUseInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errItemUseTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errItemUseFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errGMPetAddInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errGMPetAddTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errGMPetAddFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errGMPetAddResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}

func (p *Account) onGMCommandReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.GMCommandReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetCommand() == nil {
		xlog.GLog.Warnf("invalid gm command request aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if character == nil || character.record == nil {
		xlog.GLog.Warnf("gm command character not found aid:%d character:%d", p.aid, req.GetCharacterUuid())
		p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online {
		xlog.GLog.Warnf("gm command character offline aid:%d character:%d", p.aid, req.GetCharacterUuid())
		p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	// 当前阶段按需求在所有运行模式开放且不校验权限白名单,
	// 但命令只能修改当前 Account actor 内已上线角色自己的档案或邮箱.
	switch command := req.GetCommand().(type) {
	case *pb.GMCommandReq_ItemAdd:
		plan, err := prepareGMItemAddPlan(character.record, command.ItemAdd)
		if err != nil {
			xlog.GLog.Warnf("prepare gm item add failed aid:%d character:%d item:%d count:%d err:%v", p.aid, req.GetCharacterUuid(), command.ItemAdd.GetItemId(), command.ItemAdd.GetCount(), err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), gmCommandResultID(err))
			return
		}
		if err := persistGMItemAddPlan(plan, p.accountRecord, character, func() error {
			return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
		}); err != nil {
			xlog.GLog.Errorf("persist gm item add failed aid:%d character:%d item:%d count:%d err:%v", p.aid, plan.characterUUID, plan.itemID, plan.addedCount, err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Internal.Code())
			return
		}
		xlog.GLog.Infof("gm command success aid:%d account:%s character:%d clientIP:%s command:item_add item:%d added:%d current:%d", p.aid, p.account, plan.characterUUID, p.clientIP, plan.itemID, plan.addedCount, plan.currentCount)
		p.sendCharacterItemChangedNotify(gateway, plan.characterUUID, map[uint32]uint64{plan.itemID: plan.currentCount})
		p.sendCharacterTaskChangedNotify(gateway, plan.characterUUID, plan.changedTaskRecordMap)
		p.sendClientRes(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Success.Code(), &pb.GMCommandRes{
			CharacterUuid: plan.characterUUID,
			Result: &pb.GMCommandRes_ItemAdd{
				ItemAdd: &pb.GMItemAddResult{
					ItemId:     plan.itemID,
					AddedCount: plan.addedCount,
				},
			},
		})
	case *pb.GMCommandReq_PetAdd:
		plan, err := prepareGMPetAddPlan(p.accountRecord, character.record, command.PetAdd)
		if err != nil {
			xlog.GLog.Warnf("prepare gm pet add failed aid:%d character:%d pet:%d grade:%s err:%v", p.aid, req.GetCharacterUuid(), command.PetAdd.GetPetId(), command.PetAdd.GetPetGrade(), err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), gmCommandResultID(err))
			return
		}
		if err := persistGMPetAddPlan(plan, p.accountRecord, character, func() error {
			return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
		}); err != nil {
			xlog.GLog.Errorf("persist gm pet add failed aid:%d character:%d pet:%d grade:%s uuid:%d err:%v", p.aid, plan.characterUUID, plan.petID, plan.petGrade, plan.petUUID, err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Internal.Code())
			return
		}
		xlog.GLog.Infof("gm command success aid:%d account:%s character:%d clientIP:%s command:pet_add pet:%d grade:%s uuid:%d", p.aid, p.account, plan.characterUUID, p.clientIP, plan.petID, plan.petGrade, plan.petUUID)
		p.sendCharacterPetChangedNotify(gateway, plan.characterUUID, []*pb.PetRecord{plan.petRecord})
		p.sendClientRes(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Success.Code(), &pb.GMCommandRes{
			CharacterUuid: plan.characterUUID,
			Result: &pb.GMCommandRes_PetAdd{
				PetAdd: &pb.GMPetAddResult{
					PetUuid:  plan.petUUID,
					PetId:    plan.petID,
					PetGrade: plan.petGrade,
				},
			},
		})
	case *pb.GMCommandReq_MailAdd:
		title, content, err := common.NormalizeSystemMailText(command.MailAdd.GetTitle(), command.MailAdd.GetContent())
		if err != nil {
			xlog.GLog.Warnf("prepare gm system mail add failed aid:%d character:%d err:%v", p.aid, req.GetCharacterUuid(), err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.InvalidArgument.Code())
			return
		}
		mailRecord, err := unaryCacheAddSystemMail(p.aid, req.GetCharacterUuid(), title, content)
		if err != nil {
			xlog.GLog.Errorf("persist gm system mail add failed aid:%d character:%d err:%v", p.aid, req.GetCharacterUuid(), err)
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), common.GRPCStatusToResultID(err))
			return
		}
		if mailRecord == nil || mailRecord.GetUuid() == 0 {
			xlog.GLog.Errorf("persist gm system mail add returned empty record aid:%d character:%d", p.aid, req.GetCharacterUuid())
			p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Internal.Code())
			return
		}
		xlog.GLog.Infof("gm command success aid:%d account:%s character:%d clientIP:%s command:mail_add mail:%d", p.aid, p.account, req.GetCharacterUuid(), p.clientIP, mailRecord.GetUuid())
		p.sendCharacterSystemMailNotify(gateway, req.GetCharacterUuid(), mailRecord)
		p.sendClientRes(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.Success.Code(), &pb.GMCommandRes{
			CharacterUuid: req.GetCharacterUuid(),
			Result: &pb.GMCommandRes_MailAdd{
				MailAdd: &pb.GMSystemMailAddResult{MailUuid: mailRecord.GetUuid()},
			},
		})
	default:
		xlog.GLog.Warnf("unsupported gm command aid:%d character:%d command:%T", p.aid, req.GetCharacterUuid(), command)
		p.sendClientErr(gateway, uint32(pb.MsgID_GMCommandRes_CMD), xerror.InvalidArgument.Code())
	}
}
