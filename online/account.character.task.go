package main

import (
	"errors"
	"fmt"
	"math"
	"time"

	"server/common/gameconfig"
	petlogic "server/common/pet"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

var (
	errTaskInvalidArgument    = errors.New("invalid task argument")
	errTaskTargetNotFound     = errors.New("task target not found")
	errTaskAlreadyExists      = errors.New("task already exists")
	errTaskFailedPrecondition = errors.New("task precondition failed")
	errTaskResourceExhausted  = errors.New("task resource exhausted")
	errTaskRecordInvalid      = errors.New("task record is invalid")
)

type taskProgressEvent struct {
	battleVictoryEnemyGroupID uint32
	// 胜利只作用于结算前已经开始的当前步骤, 不能被后续新步骤重复消费.
	battleVictoryStepIDs map[uint32]uint32
}

type characterTaskManager struct {
	record   *pb.CharacterRecord
	usedUUID uint64
}

type characterTaskMutationPlan struct {
	characterUUID        uint64
	previous             *pb.CharacterRecord
	next                 *pb.CharacterRecord
	changedTaskRecordMap map[uint32]*pb.CharacterTaskRecord
	changedItemCountMap  map[uint32]uint64
	previousUsedUUID     uint64
	nextUsedUUID         uint64
	inventoryChanged     bool
}

func newCharacterTaskManager(record *pb.CharacterRecord) *characterTaskManager {
	return &characterTaskManager{record: record}
}

func newCharacterTaskManagerForAccount(record *pb.CharacterRecord, usedUUID uint64) *characterTaskManager {
	return &characterTaskManager{record: record, usedUUID: usedUUID}
}

func (p *characterTaskManager) Accept(taskID uint32, nowMs int64) (map[uint32]*pb.CharacterTaskRecord, error) {
	if err := p.validateMutationInput(taskID, nowMs); err != nil {
		return nil, err
	}
	task := gameconfig.GGameConfig.Task.Get(taskID)
	if task == nil {
		return nil, fmt.Errorf("%w: task %d", errTaskTargetNotFound, taskID)
	}
	if _, exists := p.record.GetTaskRecordMap()[taskID]; exists {
		return nil, fmt.Errorf("%w: task %d", errTaskAlreadyExists, taskID)
	}
	accepted, err := p.conditionsMet(task.AcceptConditions, taskProgressEvent{})
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, fmt.Errorf("%w: task %d accept conditions are not met", errTaskFailedPrecondition, taskID)
	}
	if p.record.TaskRecordMap == nil {
		p.record.TaskRecordMap = make(map[uint32]*pb.CharacterTaskRecord)
	}
	stepRecords := make([]*pb.CharacterTaskStepRecord, len(task.Steps))
	for index, step := range task.Steps {
		stepRecords[index] = &pb.CharacterTaskStepRecord{StepId: *step.ID}
	}
	p.record.TaskRecordMap[taskID] = &pb.CharacterTaskRecord{
		AcceptedAtMs:   nowMs,
		StepRecordList: stepRecords,
	}
	changed := map[uint32]*pb.CharacterTaskRecord{taskID: nil}
	automaticChanged, err := p.advanceAutomatic(nowMs, taskProgressEvent{})
	if err != nil {
		return nil, err
	}
	mergeChangedTaskRecords(changed, automaticChanged)
	return p.cloneChangedTaskRecords(changed), nil
}

func (p *characterTaskManager) Submit(taskID uint32, stepID uint32, nowMs int64) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error) {
	if stepID == 0 {
		return nil, nil, errTaskInvalidArgument
	}
	if err := p.validateMutationInput(taskID, nowMs); err != nil {
		return nil, nil, err
	}
	changed, err := p.advanceAutomatic(nowMs, taskProgressEvent{})
	if err != nil {
		return nil, nil, err
	}
	_, taskRecord, step, stepRecord, err := p.currentStep(taskID)
	if err != nil {
		return nil, nil, err
	}
	if stepRecord.GetStepId() != stepID {
		return nil, nil, fmt.Errorf("%w: task %d current step is %d, got %d", errTaskFailedPrecondition, taskID, stepRecord.GetStepId(), stepID)
	}
	if stepRecord.GetStartedAtMs() == 0 || step.CompletionMode == nil || *step.CompletionMode != gameconfig.TaskCompletionModeSubmit {
		return nil, nil, fmt.Errorf("%w: task %d step %d is not ready for submit", errTaskFailedPrecondition, taskID, stepID)
	}
	completed, err := p.conditionsMet(step.CompletionConditions, taskProgressEvent{})
	if err != nil {
		return nil, nil, err
	}
	if !completed {
		return nil, nil, fmt.Errorf("%w: task %d step %d completion conditions are not met", errTaskFailedPrecondition, taskID, stepID)
	}
	itemManager := newCharacterItemManager(p.record)
	consumedEquipmentUUIDs, err := p.findEquipmentUUIDs(step.ConsumeItems)
	if err != nil {
		return nil, nil, err
	}
	consumedPetUUIDs, err := p.findPetUUIDs(step.ConsumePets)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range step.ConsumeItems {
		if isTaskEquipmentID(*item.ItemID) {
			continue
		}
		if itemManager.Count(*item.ItemID) < *item.Quantity {
			return nil, nil, fmt.Errorf("%w: task %d step %d item %d is insufficient", errTaskFailedPrecondition, taskID, stepID, *item.ItemID)
		}
	}
	changedItemCountMap := make(map[uint32]uint64, len(step.ConsumeItems))
	for _, item := range step.ConsumeItems {
		if isTaskEquipmentID(*item.ItemID) {
			continue
		}
		if err := itemManager.Consume(*item.ItemID, *item.Quantity); err != nil {
			return nil, nil, fmt.Errorf("%w: consume task item %d: %v", errTaskRecordInvalid, *item.ItemID, err)
		}
		changedItemCountMap[*item.ItemID] = itemManager.Count(*item.ItemID)
	}
	for equipmentUUID := range consumedEquipmentUUIDs {
		delete(p.record.GetItemBag().EquipmentRecordMap, equipmentUUID)
	}
	if len(consumedPetUUIDs) > 0 {
		kept := make([]*pb.PetRecord, 0, len(p.record.GetPetRecordList())-len(consumedPetUUIDs))
		for _, petRecord := range p.record.GetPetRecordList() {
			if _, consumed := consumedPetUUIDs[petRecord.GetUuid()]; !consumed {
				kept = append(kept, petRecord)
			}
		}
		p.record.PetRecordList = kept
	}
	completeTaskStep(step, stepRecord, nowMs)
	if changed == nil {
		changed = make(map[uint32]*pb.CharacterTaskRecord)
	}
	changed[taskID] = taskRecord
	automaticChanged, err := p.advanceAutomatic(nowMs, taskProgressEvent{})
	if err != nil {
		return nil, nil, err
	}
	mergeChangedTaskRecords(changed, automaticChanged)
	return p.cloneChangedTaskRecords(changed), changedItemCountMap, nil
}

// 挑战读取所选步骤而不是当前未完成步骤, 已完成步骤可重打但不修改任何任务或领奖记录.
func (p *characterTaskManager) BattleChallengeEnemyGroupID(taskID uint32, stepID uint32) (uint32, error) {
	if stepID == 0 {
		return 0, errTaskInvalidArgument
	}
	if err := p.validateMutationInput(taskID, time.Now().UnixMilli()); err != nil {
		return 0, err
	}
	task := gameconfig.GGameConfig.Task.Get(taskID)
	taskRecord := p.record.GetTaskRecordMap()[taskID]
	if task == nil || taskRecord == nil {
		return 0, fmt.Errorf("%w: task %d", errTaskTargetNotFound, taskID)
	}
	if len(task.Steps) != len(taskRecord.GetStepRecordList()) {
		return 0, fmt.Errorf("%w: task %d step count mismatch", errTaskRecordInvalid, taskID)
	}
	if uint64(stepID) > uint64(len(task.Steps)) {
		return 0, fmt.Errorf("%w: task %d step %d", errTaskTargetNotFound, taskID, stepID)
	}
	step := task.Steps[stepID-1]
	stepRecord := taskRecord.GetStepRecordList()[stepID-1]
	if step == nil || step.ID == nil || *step.ID != stepID || stepRecord == nil || stepRecord.GetStepId() != stepID {
		return 0, fmt.Errorf("%w: task %d step %d config or record mismatch", errTaskRecordInvalid, taskID, stepID)
	}
	if stepRecord.GetStartedAtMs() == 0 || step.Challenge == nil ||
		step.Challenge.EnemyGroupID == nil || *step.Challenge.EnemyGroupID == 0 {
		return 0, fmt.Errorf("%w: task %d step %d is not challengeable", errTaskFailedPrecondition, taskID, stepID)
	}
	return *step.Challenge.EnemyGroupID, nil
}

func (p *characterTaskManager) ClaimStepReward(taskID uint32, stepID uint32, nowMs int64) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error) {
	if stepID == 0 {
		return nil, nil, errTaskInvalidArgument
	}
	if err := p.validateMutationInput(taskID, nowMs); err != nil {
		return nil, nil, err
	}
	task := gameconfig.GGameConfig.Task.Get(taskID)
	taskRecord := p.record.GetTaskRecordMap()[taskID]
	if task == nil || taskRecord == nil {
		return nil, nil, fmt.Errorf("%w: task %d", errTaskTargetNotFound, taskID)
	}
	if int(stepID) > len(task.Steps) || int(stepID) > len(taskRecord.GetStepRecordList()) {
		return nil, nil, fmt.Errorf("%w: task %d step %d", errTaskTargetNotFound, taskID, stepID)
	}
	step := task.Steps[stepID-1]
	stepRecord := taskRecord.GetStepRecordList()[stepID-1]
	if step == nil || stepRecord == nil || stepRecord.GetStepId() != stepID {
		return nil, nil, fmt.Errorf("%w: task %d step %d config or record mismatch", errTaskRecordInvalid, taskID, stepID)
	}
	if stepRecord.GetCompletedAtMs() == 0 || stepRecord.GetRewardClaimedAtMs() != 0 || step.RewardID == nil || *step.RewardID == 0 {
		return nil, nil, fmt.Errorf("%w: task %d step %d has no claimable reward", errTaskFailedPrecondition, taskID, stepID)
	}
	reward := gameconfig.GGameConfig.Reward.Get(*step.RewardID)
	if reward == nil {
		return nil, nil, fmt.Errorf("%w: reward %d", errTaskRecordInvalid, *step.RewardID)
	}
	if err := p.validateRewardCapacity(reward); err != nil {
		return nil, nil, err
	}
	itemManager := newCharacterItemManager(p.record)
	changedItemCountMap := make(map[uint32]uint64, len(reward.Items))
	for _, item := range reward.Items {
		if isTaskEquipmentID(*item.ItemID) {
			if p.record.ItemBag == nil {
				p.record.ItemBag = &pb.ItemContainerRecord{}
			}
			if p.record.ItemBag.EquipmentRecordMap == nil {
				p.record.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
			}
			for quantity := uint64(0); quantity < *item.Quantity; quantity++ {
				equipmentUUID, err := p.allocateUUID()
				if err != nil {
					return nil, nil, err
				}
				equipmentRecord, err := newEquipmentRecord(equipmentUUID, *item.ItemID)
				if err != nil {
					return nil, nil, fmt.Errorf("%w: create reward equipment %d: %v", errTaskRecordInvalid, *item.ItemID, err)
				}
				p.record.ItemBag.EquipmentRecordMap[equipmentUUID] = equipmentRecord
			}
			continue
		}
		if err := itemManager.Add(*item.ItemID, *item.Quantity); err != nil {
			if errors.Is(err, errItemUseFailedPrecondition) {
				return nil, nil, fmt.Errorf("%w: add reward item %d: %v", errTaskResourceExhausted, *item.ItemID, err)
			}
			return nil, nil, fmt.Errorf("%w: add reward item %d: %v", errTaskRecordInvalid, *item.ItemID, err)
		}
		changedItemCountMap[*item.ItemID] = itemManager.Count(*item.ItemID)
	}
	for _, rewardPet := range reward.Pets {
		petEntry := gameconfig.GGameConfig.Pet.Get(*rewardPet.PetID)
		for quantity := uint32(0); quantity < *rewardPet.Quantity; quantity++ {
			petUUID, err := p.allocateUUID()
			if err != nil {
				return nil, nil, err
			}
			petRecord, err := petlogic.NewRecord(petEntry, petUUID, *rewardPet.Level, pb.PetGrade_PetGrade_Unknow)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: create reward pet %d: %v", errTaskRecordInvalid, *rewardPet.PetID, err)
			}
			petRecord.CarryStatus = pb.PetCarryStatus_PetCarryStatus_Wait
			petRecord.CreateTimestampMs = nowMs
			p.record.PetRecordList = append(p.record.PetRecordList, petRecord)
		}
	}
	stepRecord.RewardClaimedAtMs = nowMs
	changed := map[uint32]*pb.CharacterTaskRecord{taskID: taskRecord}
	if task.Repeatable != nil && *task.Repeatable && characterTaskRewardsClaimed(p.record, taskID) {
		resetCharacterTaskRecord(task, taskRecord, nowMs)
	}
	automaticChanged, err := p.advanceAutomatic(nowMs, taskProgressEvent{})
	if err != nil {
		return nil, nil, err
	}
	mergeChangedTaskRecords(changed, automaticChanged)
	return p.cloneChangedTaskRecords(changed), changedItemCountMap, nil
}

func (p *characterTaskManager) HandleBattleVictory(enemyGroupID uint32, nowMs int64) (map[uint32]*pb.CharacterTaskRecord, error) {
	if enemyGroupID == 0 {
		return nil, nil
	}
	if err := p.validateRecordInput(nowMs); err != nil {
		return nil, err
	}
	eligibleSteps := make(map[uint32]uint32)
	for taskID, taskRecord := range p.record.GetTaskRecordMap() {
		for _, stepRecord := range taskRecord.GetStepRecordList() {
			if stepRecord.GetCompletedAtMs() != 0 {
				continue
			}
			if stepRecord.GetStartedAtMs() != 0 {
				eligibleSteps[taskID] = stepRecord.GetStepId()
			}
			break
		}
	}
	return p.advanceAutomatic(nowMs, taskProgressEvent{
		battleVictoryEnemyGroupID: enemyGroupID,
		battleVictoryStepIDs:      eligibleSteps,
	})
}

// Refresh检查角色等级、持有道具、随身宠物及前置任务等当前状态条件.
// 调用者在本次状态变更的候选档案上调用, 并与原业务共用持久化和回滚.
func (p *characterTaskManager) Refresh(nowMs int64) (map[uint32]*pb.CharacterTaskRecord, error) {
	if p == nil || p.record == nil || nowMs <= 0 {
		return nil, errTaskInvalidArgument
	}
	if len(p.record.GetTaskRecordMap()) == 0 {
		return nil, nil
	}
	if err := p.validateRecordInput(nowMs); err != nil {
		return nil, err
	}
	return p.advanceAutomatic(nowMs, taskProgressEvent{})
}

func (p *characterTaskManager) validateMutationInput(taskID uint32, nowMs int64) error {
	if taskID == 0 {
		return errTaskInvalidArgument
	}
	return p.validateRecordInput(nowMs)
}

func (p *characterTaskManager) validateRecordInput(nowMs int64) error {
	if p == nil || p.record == nil || p.record.GetBase() == nil || p.record.GetBase().GetUuid() == 0 || nowMs <= 0 {
		return errTaskInvalidArgument
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Task == nil || gameconfig.GGameConfig.Reward == nil || gameconfig.GGameConfig.Exp == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Pet == nil {
		return fmt.Errorf("%w: task dependencies are not loaded", errTaskRecordInvalid)
	}
	return nil
}

func (p *characterTaskManager) currentStep(taskID uint32) (*gameconfig.TaskEntry, *pb.CharacterTaskRecord, *gameconfig.TaskStepEntry, *pb.CharacterTaskStepRecord, error) {
	task := gameconfig.GGameConfig.Task.Get(taskID)
	taskRecord := p.record.GetTaskRecordMap()[taskID]
	if task == nil || taskRecord == nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: task %d", errTaskTargetNotFound, taskID)
	}
	if len(task.Steps) != len(taskRecord.GetStepRecordList()) {
		return nil, nil, nil, nil, fmt.Errorf("%w: task %d step count mismatch", errTaskRecordInvalid, taskID)
	}
	for index, stepRecord := range taskRecord.GetStepRecordList() {
		step := task.Steps[index]
		if step == nil || step.ID == nil || stepRecord == nil || stepRecord.GetStepId() != *step.ID {
			return nil, nil, nil, nil, fmt.Errorf("%w: task %d step %d mismatch", errTaskRecordInvalid, taskID, index+1)
		}
		if stepRecord.GetCompletedAtMs() == 0 {
			return task, taskRecord, step, stepRecord, nil
		}
	}
	return nil, nil, nil, nil, fmt.Errorf("%w: task %d is already completed", errTaskFailedPrecondition, taskID)
}

func (p *characterTaskManager) advanceAutomatic(nowMs int64, event taskProgressEvent) (map[uint32]*pb.CharacterTaskRecord, error) {
	changedIDs := make(map[uint32]*pb.CharacterTaskRecord)
	for {
		iterationChanged := false
		var iterationErr error
		gameconfig.GGameConfig.Task.Foreach(func(taskID uint32, task *gameconfig.TaskEntry) bool {
			taskRecord := p.record.GetTaskRecordMap()[taskID]
			if taskRecord == nil {
				return true
			}
			changed, err := p.advanceTask(taskID, task, taskRecord, nowMs, event)
			if err != nil {
				iterationErr = err
				return false
			}
			if changed {
				iterationChanged = true
				changedIDs[taskID] = taskRecord
			}
			return true
		})
		if iterationErr != nil {
			return nil, iterationErr
		}
		if !iterationChanged {
			break
		}
	}
	return p.cloneChangedTaskRecords(changedIDs), nil
}

func (p *characterTaskManager) advanceTask(taskID uint32, task *gameconfig.TaskEntry, taskRecord *pb.CharacterTaskRecord, nowMs int64, event taskProgressEvent) (bool, error) {
	if task == nil || taskRecord == nil || len(task.Steps) != len(taskRecord.GetStepRecordList()) {
		return false, fmt.Errorf("%w: task %d config or record is incomplete", errTaskRecordInvalid, taskID)
	}
	changed := false
	for index, step := range task.Steps {
		stepRecord := taskRecord.GetStepRecordList()[index]
		if step == nil || step.ID == nil || stepRecord == nil || stepRecord.GetStepId() != *step.ID {
			return false, fmt.Errorf("%w: task %d step %d mismatch", errTaskRecordInvalid, taskID, index+1)
		}
		if stepRecord.GetCompletedAtMs() != 0 {
			continue
		}
		if stepRecord.GetStartedAtMs() == 0 {
			ready, err := p.conditionsMet(step.StartConditions, event)
			if err != nil {
				return false, err
			}
			if !ready {
				return changed, nil
			}
			stepRecord.StartedAtMs = nowMs
			changed = true
		}
		if step.CompletionMode == nil || *step.CompletionMode != gameconfig.TaskCompletionModeAutomatic {
			return changed, nil
		}
		completionEvent := event
		if event.battleVictoryStepIDs[taskID] != *step.ID {
			completionEvent.battleVictoryEnemyGroupID = 0
		}
		ready, err := p.conditionsMet(step.CompletionConditions, completionEvent)
		if err != nil {
			return false, err
		}
		if !ready {
			return changed, nil
		}
		completeTaskStep(step, stepRecord, nowMs)
		changed = true
	}
	return changed, nil
}

func (p *characterTaskManager) conditionsMet(conditions []gameconfig.TaskConditionEntry, event taskProgressEvent) (bool, error) {
	for _, condition := range conditions {
		if condition.Kind == nil {
			return false, fmt.Errorf("%w: task condition kind is nil", errTaskRecordInvalid)
		}
		switch *condition.Kind {
		case gameconfig.TaskConditionKindCharacterLevel:
			level, err := gameconfig.GGameConfig.Exp.GetLevel(p.record.GetBase().GetExp())
			if err != nil {
				return false, fmt.Errorf("%w: derive character level: %v", errTaskRecordInvalid, err)
			}
			if condition.Level == nil || level < *condition.Level {
				return false, nil
			}
		case gameconfig.TaskConditionKindItemPossession:
			if condition.ItemID == nil || condition.Quantity == nil || p.itemCount(*condition.ItemID) < *condition.Quantity {
				return false, nil
			}
		case gameconfig.TaskConditionKindPetPossession:
			if condition.PetID == nil || condition.Level == nil || condition.Quantity == nil {
				return false, nil
			}
			count, err := p.petCount(*condition.PetID, *condition.Level)
			if err != nil {
				return false, err
			}
			if count < *condition.Quantity {
				return false, nil
			}
		case gameconfig.TaskConditionKindTaskCompleted:
			if condition.TaskID == nil || !characterTaskCompleted(p.record, *condition.TaskID) {
				return false, nil
			}
		case gameconfig.TaskConditionKindTaskRewardsClaimed:
			if condition.TaskID == nil || !characterTaskRewardsClaimed(p.record, *condition.TaskID) {
				return false, nil
			}
		case gameconfig.TaskConditionKindBattleVictory:
			if condition.EnemyGroupID == nil || event.battleVictoryEnemyGroupID != *condition.EnemyGroupID {
				return false, nil
			}
		default:
			return false, fmt.Errorf("%w: task condition kind %q", errTaskRecordInvalid, *condition.Kind)
		}
	}
	return true, nil
}

func characterTaskCompleted(record *pb.CharacterRecord, taskID uint32) bool {
	if record == nil || taskID == 0 || gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Task == nil {
		return false
	}
	task := gameconfig.GGameConfig.Task.Get(taskID)
	taskRecord := record.GetTaskRecordMap()[taskID]
	if task == nil || taskRecord == nil || len(task.Steps) == 0 || len(task.Steps) != len(taskRecord.GetStepRecordList()) {
		return false
	}
	for index, stepRecord := range taskRecord.GetStepRecordList() {
		if stepRecord == nil || stepRecord.GetStepId() != uint32(index+1) || stepRecord.GetCompletedAtMs() == 0 {
			return false
		}
	}
	return true
}

func characterTaskRewardsClaimed(record *pb.CharacterRecord, taskID uint32) bool {
	if !characterTaskCompleted(record, taskID) {
		return false
	}
	for _, stepRecord := range record.GetTaskRecordMap()[taskID].GetStepRecordList() {
		if stepRecord.GetRewardClaimedAtMs() == 0 {
			return false
		}
	}
	return true
}

func resetCharacterTaskRecord(task *gameconfig.TaskEntry, taskRecord *pb.CharacterTaskRecord, nowMs int64) {
	taskRecord.AcceptedAtMs = nowMs
	taskRecord.StepRecordList = make([]*pb.CharacterTaskStepRecord, len(task.Steps))
	for index, step := range task.Steps {
		taskRecord.StepRecordList[index] = &pb.CharacterTaskStepRecord{StepId: *step.ID}
	}
}

func completeTaskStep(step *gameconfig.TaskStepEntry, stepRecord *pb.CharacterTaskStepRecord, nowMs int64) {
	stepRecord.CompletedAtMs = nowMs
	if step.RewardID != nil && *step.RewardID == 0 {
		stepRecord.RewardClaimedAtMs = nowMs
	}
}

func isTaskEquipmentID(itemID uint32) bool {
	return itemID >= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Start) &&
		itemID <= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_End)
}

func (p *characterTaskManager) itemCount(itemID uint32) uint64 {
	if !isTaskEquipmentID(itemID) {
		return newCharacterItemManager(p.record).Count(itemID)
	}
	var count uint64
	for _, equipmentRecord := range p.record.GetItemBag().GetEquipmentRecordMap() {
		if equipmentRecord.GetAssetId() == itemID {
			count++
		}
	}
	return count
}

func (p *characterTaskManager) petCount(petID uint32, level uint32) (uint64, error) {
	var count uint64
	for _, petRecord := range p.record.GetPetRecordList() {
		if petRecord.GetAssetId() != petID {
			continue
		}
		petLevel, err := gameconfig.GGameConfig.Exp.GetLevel(petRecord.GetExp())
		if err != nil {
			return 0, fmt.Errorf("%w: derive pet %d level: %v", errTaskRecordInvalid, petRecord.GetUuid(), err)
		}
		if petLevel == level {
			count++
		}
	}
	return count, nil
}

func (p *characterTaskManager) findEquipmentUUIDs(items []gameconfig.TaskItemEntry) (map[uint64]struct{}, error) {
	selected := make(map[uint64]struct{})
	for _, item := range items {
		if !isTaskEquipmentID(*item.ItemID) {
			continue
		}
		remaining := *item.Quantity
		for equipmentUUID, equipmentRecord := range p.record.GetItemBag().GetEquipmentRecordMap() {
			if equipmentRecord.GetAssetId() != *item.ItemID {
				continue
			}
			selected[equipmentUUID] = struct{}{}
			remaining--
			if remaining == 0 {
				break
			}
		}
		if remaining != 0 {
			return nil, fmt.Errorf("%w: task equipment %d is insufficient", errTaskFailedPrecondition, *item.ItemID)
		}
	}
	return selected, nil
}

func (p *characterTaskManager) findPetUUIDs(pets []gameconfig.TaskPetEntry) (map[uint64]struct{}, error) {
	selected := make(map[uint64]struct{})
	for _, pet := range pets {
		remaining := *pet.Quantity
		for _, petRecord := range p.record.GetPetRecordList() {
			if petRecord.GetAssetId() != *pet.PetID {
				continue
			}
			level, err := gameconfig.GGameConfig.Exp.GetLevel(petRecord.GetExp())
			if err != nil {
				return nil, fmt.Errorf("%w: derive pet %d level: %v", errTaskRecordInvalid, petRecord.GetUuid(), err)
			}
			if level != *pet.Level {
				continue
			}
			selected[petRecord.GetUuid()] = struct{}{}
			remaining--
			if remaining == 0 {
				break
			}
		}
		if remaining != 0 {
			return nil, fmt.Errorf("%w: task pet %d level %d is insufficient", errTaskFailedPrecondition, *pet.PetID, *pet.Level)
		}
	}
	return selected, nil
}

func (p *characterTaskManager) validateRewardCapacity(reward *gameconfig.RewardEntry) error {
	if reward == nil {
		return fmt.Errorf("%w: reward is nil", errTaskRecordInvalid)
	}
	petCount := uint64(len(p.record.GetPetRecordList()))
	for _, rewardPet := range reward.Pets {
		petCount += uint64(*rewardPet.Quantity)
	}
	if petCount > uint64(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return fmt.Errorf("%w: pet carry capacity exhausted", errTaskResourceExhausted)
	}
	bagCount := uint64(itemContainerCount(p.record.GetItemBag()))
	for _, item := range reward.Items {
		if isTaskEquipmentID(*item.ItemID) {
			bagCount += *item.Quantity
			continue
		}
		if !isCharacterAssetItemID(*item.ItemID) && newCharacterItemManager(p.record).Count(*item.ItemID) == 0 {
			bagCount++
		}
	}
	if bagCount > uint64(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
		return fmt.Errorf("%w: item bag capacity exhausted", errTaskResourceExhausted)
	}
	return nil
}

func (p *characterTaskManager) allocateUUID() (uint64, error) {
	if p.usedUUID == math.MaxUint64 {
		return 0, fmt.Errorf("%w: account uuid exhausted", errTaskResourceExhausted)
	}
	p.usedUUID++
	return p.usedUUID, nil
}

func mergeChangedTaskRecords(target map[uint32]*pb.CharacterTaskRecord, source map[uint32]*pb.CharacterTaskRecord) {
	for taskID, record := range source {
		target[taskID] = record
	}
}

func cloneCharacterTaskRecordMap(source map[uint32]*pb.CharacterTaskRecord) map[uint32]*pb.CharacterTaskRecord {
	if source == nil {
		return nil
	}
	result := make(map[uint32]*pb.CharacterTaskRecord, len(source))
	for taskID, taskRecord := range source {
		if taskRecord != nil {
			result[taskID] = proto.Clone(taskRecord).(*pb.CharacterTaskRecord)
		}
	}
	return result
}

func (p *characterTaskManager) cloneChangedTaskRecords(changed map[uint32]*pb.CharacterTaskRecord) map[uint32]*pb.CharacterTaskRecord {
	if len(changed) == 0 {
		return nil
	}
	result := make(map[uint32]*pb.CharacterTaskRecord, len(changed))
	for taskID := range changed {
		taskRecord := p.record.GetTaskRecordMap()[taskID]
		if taskRecord != nil {
			result[taskID] = proto.Clone(taskRecord).(*pb.CharacterTaskRecord)
		}
	}
	return result
}

func prepareCharacterTaskAcceptPlan(accountRecord *pb.AccountRecord, record *pb.CharacterRecord, taskID uint32, nowMs int64) (*characterTaskMutationPlan, error) {
	return prepareCharacterTaskMutationPlan(accountRecord, record, func(manager *characterTaskManager) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error) {
		changed, err := manager.Accept(taskID, nowMs)
		return changed, nil, err
	})
}

func prepareCharacterTaskSubmitPlan(accountRecord *pb.AccountRecord, record *pb.CharacterRecord, taskID uint32, stepID uint32, nowMs int64) (*characterTaskMutationPlan, error) {
	return prepareCharacterTaskMutationPlan(accountRecord, record, func(manager *characterTaskManager) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error) {
		return manager.Submit(taskID, stepID, nowMs)
	})
}

func prepareCharacterTaskRewardClaimPlan(accountRecord *pb.AccountRecord, record *pb.CharacterRecord, taskID uint32, stepID uint32, nowMs int64) (*characterTaskMutationPlan, error) {
	return prepareCharacterTaskMutationPlan(accountRecord, record, func(manager *characterTaskManager) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error) {
		return manager.ClaimStepReward(taskID, stepID, nowMs)
	})
}

func prepareCharacterTaskMutationPlan(
	accountRecord *pb.AccountRecord,
	record *pb.CharacterRecord,
	mutate func(*characterTaskManager) (map[uint32]*pb.CharacterTaskRecord, map[uint32]uint64, error),
) (*characterTaskMutationPlan, error) {
	if accountRecord == nil || record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 || mutate == nil {
		return nil, errTaskInvalidArgument
	}
	next := proto.Clone(record).(*pb.CharacterRecord)
	manager := newCharacterTaskManagerForAccount(next, accountRecord.GetUsedUuid())
	changedTasks, changedItems, err := mutate(manager)
	if err != nil {
		return nil, err
	}
	if len(changedTasks) == 0 {
		return nil, fmt.Errorf("%w: task mutation produced no task change", errTaskRecordInvalid)
	}
	return &characterTaskMutationPlan{
		characterUUID:        record.GetBase().GetUuid(),
		previous:             record,
		next:                 next,
		changedTaskRecordMap: changedTasks,
		changedItemCountMap:  changedItems,
		previousUsedUUID:     accountRecord.GetUsedUuid(),
		nextUsedUUID:         manager.usedUUID,
		inventoryChanged:     len(changedItems) > 0 || manager.usedUUID != accountRecord.GetUsedUuid() || !proto.Equal(record.GetItemBag(), next.GetItemBag()) || !petRecordListsEqual(record.GetPetRecordList(), next.GetPetRecordList()),
	}, nil
}

func petRecordListsEqual(left []*pb.PetRecord, right []*pb.PetRecord) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !proto.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}

func persistCharacterTaskMutationPlan(
	plan *characterTaskMutationPlan,
	accountRecord *pb.AccountRecord,
	character *character,
	persist func() error,
) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.previous == nil || plan.next == nil {
		return errTaskInvalidArgument
	}
	characterSlot := -1
	for index, record := range accountRecord.GetCharacterRecordList() {
		if record == plan.previous && record.GetBase().GetUuid() == plan.characterUUID {
			characterSlot = index
			break
		}
	}
	if characterSlot < 0 || character.record != plan.previous || accountRecord.GetUsedUuid() != plan.previousUsedUUID {
		return fmt.Errorf("%w: character %d record slot not found", errTaskRecordInvalid, plan.characterUUID)
	}
	accountRecord.CharacterRecordList[characterSlot] = plan.next
	accountRecord.UsedUuid = plan.nextUsedUUID
	if err := persist(); err != nil {
		accountRecord.CharacterRecordList[characterSlot] = plan.previous
		accountRecord.UsedUuid = plan.previousUsedUUID
		return err
	}
	character.record = plan.next
	return nil
}

// 任务挑战只校验任务状态并进入既有PVE建房流程, 不要求地图NPC在场,
// 不接收客户端自报敌群或BGM, 也不在点击时修改任务进度.
func (p *Account) onTaskBattleChallengeReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.TaskBattleChallengeReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetTaskId() == 0 || req.GetStepId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskBattleChallengeRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if resultID := validateTaskCharacterState(character); resultID != 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskBattleChallengeRes_CMD), resultID)
		return
	}
	enemyGroupID, err := newCharacterTaskManager(character.record).BattleChallengeEnemyGroupID(req.GetTaskId(), req.GetStepId())
	if err != nil {
		p.logAndSendTaskError(gateway, uint32(pb.MsgID_TaskBattleChallengeRes_CMD), req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), err)
		return
	}
	if err := character.startCombatPVE(gateway, enemyGroupID); err != nil {
		xlog.GLog.Warnf("task battle challenge failed aid:%d character:%d task:%d step:%d enemyGroup:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), enemyGroupID, err)
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskBattleChallengeRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_TaskBattleChallengeRes_CMD), xerror.Success.Code(), &pb.TaskBattleChallengeRes{
		CharacterUuid: req.GetCharacterUuid(), TaskId: req.GetTaskId(), StepId: req.GetStepId(),
	})
}

func (p *Account) onTaskAcceptReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.TaskAcceptReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetTaskId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskAcceptRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if resultID := validateTaskCharacterState(character); resultID != 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskAcceptRes_CMD), resultID)
		return
	}
	plan, err := prepareCharacterTaskAcceptPlan(p.accountRecord, character.record, req.GetTaskId(), time.Now().UnixMilli())
	if err != nil {
		p.logAndSendTaskError(gateway, uint32(pb.MsgID_TaskAcceptRes_CMD), req.GetCharacterUuid(), req.GetTaskId(), 0, err)
		return
	}
	if err := persistCharacterTaskMutationPlan(plan, p.accountRecord, character, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist task accept failed aid:%d character:%d task:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetTaskId(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskAcceptRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendCharacterTaskChangedNotify(gateway, plan.characterUUID, plan.changedTaskRecordMap)
	p.sendClientRes(gateway, uint32(pb.MsgID_TaskAcceptRes_CMD), xerror.Success.Code(), &pb.TaskAcceptRes{CharacterUuid: plan.characterUUID, TaskId: req.GetTaskId()})
}

func (p *Account) onTaskSubmitReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.TaskSubmitReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetTaskId() == 0 || req.GetStepId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskSubmitRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if resultID := validateTaskCharacterState(character); resultID != 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskSubmitRes_CMD), resultID)
		return
	}
	plan, err := prepareCharacterTaskSubmitPlan(p.accountRecord, character.record, req.GetTaskId(), req.GetStepId(), time.Now().UnixMilli())
	if err != nil {
		p.logAndSendTaskError(gateway, uint32(pb.MsgID_TaskSubmitRes_CMD), req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), err)
		return
	}
	if err := persistCharacterTaskMutationPlan(plan, p.accountRecord, character, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist task submit failed aid:%d character:%d task:%d step:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskSubmitRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendCharacterTaskInventoryChangedNotify(gateway, plan)
	p.sendCharacterTaskChangedNotify(gateway, plan.characterUUID, plan.changedTaskRecordMap)
	if plan.inventoryChanged {
		p.refreshCharacterPresence(character)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_TaskSubmitRes_CMD), xerror.Success.Code(), &pb.TaskSubmitRes{CharacterUuid: plan.characterUUID, TaskId: req.GetTaskId(), StepId: req.GetStepId()})
}

func (p *Account) onTaskStepRewardClaimReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.TaskStepRewardClaimReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil || req.GetCharacterUuid() == 0 || req.GetTaskId() == 0 || req.GetStepId() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskStepRewardClaimRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(req.GetCharacterUuid())
	if resultID := validateTaskCharacterState(character); resultID != 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskStepRewardClaimRes_CMD), resultID)
		return
	}
	plan, err := prepareCharacterTaskRewardClaimPlan(p.accountRecord, character.record, req.GetTaskId(), req.GetStepId(), time.Now().UnixMilli())
	if err != nil {
		p.logAndSendTaskError(gateway, uint32(pb.MsgID_TaskStepRewardClaimRes_CMD), req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), err)
		return
	}
	if err := persistCharacterTaskMutationPlan(plan, p.accountRecord, character, func() error {
		return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
	}); err != nil {
		xlog.GLog.Errorf("persist task reward claim failed aid:%d character:%d task:%d step:%d err:%v", p.aid, req.GetCharacterUuid(), req.GetTaskId(), req.GetStepId(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_TaskStepRewardClaimRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendCharacterTaskInventoryChangedNotify(gateway, plan)
	p.sendCharacterTaskChangedNotify(gateway, plan.characterUUID, plan.changedTaskRecordMap)
	if plan.inventoryChanged {
		p.refreshCharacterPresence(character)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_TaskStepRewardClaimRes_CMD), xerror.Success.Code(), &pb.TaskStepRewardClaimRes{CharacterUuid: plan.characterUUID, TaskId: req.GetTaskId(), StepId: req.GetStepId()})
}

func validateTaskCharacterState(character *character) uint32 {
	if character == nil || character.record == nil {
		return xerror.NotFound.Code()
	}
	if !character.online || character.combatRoom != nil {
		return xerror.FailedPrecondition.Code()
	}
	return 0
}

func (p *Account) logAndSendTaskError(gateway *Gateway, responseID uint32, characterUUID uint64, taskID uint32, stepID uint32, err error) {
	xlog.GLog.Warnf("task mutation rejected aid:%d character:%d task:%d step:%d err:%v", p.aid, characterUUID, taskID, stepID, err)
	p.sendClientErr(gateway, responseID, taskResultID(err))
}

func taskResultID(err error) uint32 {
	switch {
	case errors.Is(err, errTaskInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errTaskTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errTaskAlreadyExists):
		return xerror.AlreadyExists.Code()
	case errors.Is(err, errTaskFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errTaskResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}
