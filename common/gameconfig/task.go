package gameconfig

import (
	"strings"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type TaskCompletionMode string

const (
	TaskCompletionModeAutomatic  TaskCompletionMode = "automatic"
	TaskCompletionModeSubmit     TaskCompletionMode = "submit"
	TaskCompletionModePersistent TaskCompletionMode = "persistent"
)

type TaskInteractionState string

type TaskNPCVisualType string

const (
	TaskInteractionStateUnavailable TaskInteractionState = "unavailable"
	TaskInteractionCharacterRebirth                      = "characterRebirth"
	TaskNPCVisualTypeCharacterMount TaskNPCVisualType    = "characterMount"
)

type TaskConditionKind string

const (
	TaskConditionKindCharacterLevel        TaskConditionKind = "characterLevel"
	TaskConditionKindItemPossession        TaskConditionKind = "itemPossession"
	TaskConditionKindPetPossession         TaskConditionKind = "petPossession"
	TaskConditionKindTaskCompleted         TaskConditionKind = "taskCompleted"
	TaskConditionKindTaskRewardsClaimed    TaskConditionKind = "taskRewardsClaimed"
	TaskConditionKindAnyTaskRewardsClaimed TaskConditionKind = "anyTaskRewardsClaimed"
	TaskConditionKindBattleVictory         TaskConditionKind = "battleVictory"
)

type TaskConfig struct {
	*xmap.MapMgr[uint32, *TaskEntry]
}

type TaskEntry struct {
	ID                               *uint32              `yaml:"id"`
	Name                             *string              `yaml:"name"`
	Description                      *string              `yaml:"description"`
	IsMain                           *bool                `yaml:"isMain"`
	Repeatable                       *bool                `yaml:"repeatable"`
	CompletionRequiresRewardsClaimed *bool                `yaml:"completionRequiresRewardsClaimed"`
	Sort                             int32                `yaml:"sort"`
	AcceptConditions                 []TaskConditionEntry `yaml:"acceptConditions"`
	Steps                            []*TaskStepEntry     `yaml:"steps"`
}

type TaskStepEntry struct {
	ID                   *uint32                `yaml:"id"`
	Name                 *string                `yaml:"name"`
	Description          *string                `yaml:"description"`
	StartConditions      []TaskConditionEntry   `yaml:"startConditions"`
	CompletionConditions []TaskConditionEntry   `yaml:"completionConditions"`
	CompletionMode       *TaskCompletionMode    `yaml:"completionMode"`
	Challenge            *TaskChallengeEntry    `yaml:"challenge"`
	Navigation           *TaskNavigationEntry   `yaml:"navigation"`
	ConsumeItems         []TaskItemEntry        `yaml:"consumeItems"`
	ConsumePets          []TaskPetEntry         `yaml:"consumePets"`
	RewardID             *uint32                `yaml:"rewardId"`
	RewardReissue        *TaskRewardReissue     `yaml:"rewardReissue"`
	Interactions         []TaskInteractionEntry `yaml:"interactions"`
	NPC                  *TaskNPCEntry          `yaml:"npc"`
}

// TaskNPCEntry只保留服务端必须校验的任务NPC形象引用. 动画方向和动作仍由客户端解释.
type TaskNPCEntry struct {
	Visual *TaskNPCVisualEntry `yaml:"visual"`
}

type TaskNPCVisualEntry struct {
	Type        *TaskNPCVisualType `yaml:"type"`
	CharacterID *uint32            `yaml:"characterId"`
	PetID       *uint32            `yaml:"petId"`
}

type TaskInteractionEntry struct {
	ID                    *string                       `yaml:"id"`
	Label                 *string                       `yaml:"label"`
	State                 *TaskInteractionState         `yaml:"state"`
	UnavailableMessage    *string                       `yaml:"unavailableMessage"`
	RequirementsByRebirth []TaskRebirthRequirementEntry `yaml:"requirementsByRebirth"`
}

type TaskRebirthRequirementEntry struct {
	RebirthCount *uint32        `yaml:"rebirthCount"`
	Pets         []TaskPetEntry `yaml:"pets"`
}

// TaskRewardReissue允许已领取的步骤奖励在任务未完成且指定道具为0时补领.
// 奖励包必须只包含该道具, 避免补领时重复发放其他奖励.
type TaskRewardReissue struct {
	WhenItemAbsent *uint32 `yaml:"whenItemAbsent"`
}

// TaskChallengeEntry只读取服务端开战所需的敌群. 同一challenge下的
// npcs和battleBgmIndex由客户端读取, 不参与服务端战斗和任务判定.
type TaskChallengeEntry struct {
	EnemyGroupID *uint32 `yaml:"enemyGroupId"`
}

// 任务地图入口可指定落点; 旧任务省略坐标时继续使用地图默认出生格.
type TaskNavigationEntry struct {
	MapID *uint32 `yaml:"mapId"`
	X     *uint32 `yaml:"x"`
	Y     *uint32 `yaml:"y"`
}

type TaskConditionEntry struct {
	Kind         *TaskConditionKind `yaml:"kind"`
	Level        *uint32            `yaml:"level"`
	ItemID       *uint32            `yaml:"itemId"`
	PetID        *uint32            `yaml:"petId"`
	Quantity     *uint64            `yaml:"quantity"`
	TaskID       *uint32            `yaml:"taskId"`
	TaskIDs      []uint32           `yaml:"taskIds"`
	EnemyGroupID *uint32            `yaml:"enemyGroupId"`
}

type TaskItemEntry struct {
	ItemID   *uint32 `yaml:"itemId"`
	Quantity *uint64 `yaml:"quantity"`
}

type TaskPetEntry struct {
	PetID    *uint32 `yaml:"petId"`
	Level    *uint32 `yaml:"level"`
	Quantity *uint32 `yaml:"quantity"`
}

func newTaskConfig() *TaskConfig {
	return &TaskConfig{MapMgr: xmap.NewMapMgr[uint32, *TaskEntry]()}
}

func (p *TaskConfig) load(dir string) error {
	var root struct {
		Tasks []*TaskEntry `yaml:"tasks"`
	}
	if err := loadYAMLFile(dir, FileTask, &root); err != nil {
		return err
	}
	return p.configure(root.Tasks)
}

func (p *TaskConfig) configure(entries []*TaskEntry) error {
	for index, task := range entries {
		if task == nil {
			return errors.Errorf("任务不能为空: index:%d %v", index, xruntime.Location())
		}
		if task.ID == nil || *task.ID == 0 {
			return errors.Errorf("任务ID必须大于0: index:%d %v", index, xruntime.Location())
		}
		if task.Name == nil || strings.TrimSpace(*task.Name) == "" {
			return errors.Errorf("任务名称不能为空: id:%d %v", *task.ID, xruntime.Location())
		}
		if task.Description == nil || strings.TrimSpace(*task.Description) == "" {
			return errors.Errorf("任务说明不能为空: id:%d %v", *task.ID, xruntime.Location())
		}
		defaultBool(&task.IsMain, false)
		defaultBool(&task.Repeatable, false)
		defaultBool(&task.CompletionRequiresRewardsClaimed, false)
		if len(task.Steps) == 0 {
			return errors.Errorf("任务步骤不能为空: id:%d %v", *task.ID, xruntime.Location())
		}
		for stepIndex, step := range task.Steps {
			if err := configureTaskStep(*task.ID, uint32(stepIndex+1), uint32(len(task.Steps)), step); err != nil {
				return err
			}
		}
		if !p.AddIfNotExist(*task.ID, task) {
			return errors.Errorf("任务ID重复: %d %v", *task.ID, xruntime.Location())
		}
	}
	return nil
}

func configureTaskStep(taskID uint32, expectedStepID uint32, stepCount uint32, step *TaskStepEntry) error {
	if step == nil {
		return errors.Errorf("任务步骤不能为空: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.ID == nil || *step.ID != expectedStepID {
		return errors.Errorf("任务步骤ID必须从1连续递增: task:%d expected:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.Name == nil || strings.TrimSpace(*step.Name) == "" {
		return errors.Errorf("任务步骤名称不能为空: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.Description == nil || strings.TrimSpace(*step.Description) == "" {
		return errors.Errorf("任务步骤说明不能为空: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.NPC != nil && step.NPC.Visual != nil && step.NPC.Visual.Type != nil && *step.NPC.Visual.Type == TaskNPCVisualTypeCharacterMount {
		visual := step.NPC.Visual
		if visual.CharacterID == nil || !isCharacterID(*visual.CharacterID) || visual.PetID == nil || !isPetID(*visual.PetID) {
			return errors.Errorf("任务骑乘NPC形象引用无效: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
		}
	}
	if step.Navigation != nil && (step.Navigation.MapID == nil || *step.Navigation.MapID == 0 || (step.Navigation.X == nil) != (step.Navigation.Y == nil)) {
		return errors.Errorf("任务步骤地图入口或坐标无效: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.CompletionMode == nil {
		step.CompletionMode = valuePtr(TaskCompletionModeAutomatic)
	}
	if *step.CompletionMode != TaskCompletionModeAutomatic && *step.CompletionMode != TaskCompletionModeSubmit && *step.CompletionMode != TaskCompletionModePersistent {
		return errors.Errorf("任务步骤completionMode无效: task:%d step:%d mode:%q %v", taskID, expectedStepID, *step.CompletionMode, xruntime.Location())
	}
	if *step.CompletionMode == TaskCompletionModePersistent {
		if expectedStepID != stepCount || len(step.CompletionConditions) != 0 || len(step.ConsumeItems) != 0 || len(step.ConsumePets) != 0 || step.Challenge != nil || len(step.Interactions) == 0 {
			return errors.Errorf("persistent只能用于无完成条件、挑战和消耗且包含交互的最后一步: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
		}
	} else if len(step.CompletionConditions) == 0 {
		return errors.Errorf("任务步骤完成条件不能为空: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.RewardID == nil {
		return errors.Errorf("任务步骤缺少rewardId: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if *step.CompletionMode == TaskCompletionModePersistent && *step.RewardID != 0 {
		return errors.Errorf("persistent步骤不能配置奖励: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if *step.CompletionMode != TaskCompletionModePersistent && len(step.Interactions) != 0 {
		return errors.Errorf("只有persistent步骤可以配置interactions: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if err := configureTaskInteractions(taskID, expectedStepID, step.Interactions); err != nil {
		return err
	}
	if step.RewardReissue != nil && (step.RewardReissue.WhenItemAbsent == nil ||
		!isItemID(*step.RewardReissue.WhenItemAbsent) || *step.RewardID == 0) {
		return errors.Errorf("任务步骤rewardReissue无效: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if (len(step.ConsumeItems) > 0 || len(step.ConsumePets) > 0) && *step.CompletionMode != TaskCompletionModeSubmit {
		return errors.Errorf("配置扣除内容的任务步骤必须主动提交: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.Challenge != nil {
		if step.Challenge.EnemyGroupID == nil || *step.Challenge.EnemyGroupID == 0 || *step.CompletionMode != TaskCompletionModeAutomatic {
			return errors.Errorf("任务挑战必须指定敌群并使用automatic: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
		}
		matchingVictory := false
		for _, condition := range step.CompletionConditions {
			if condition.Kind != nil && *condition.Kind == TaskConditionKindBattleVictory &&
				condition.EnemyGroupID != nil && *condition.EnemyGroupID == *step.Challenge.EnemyGroupID {
				matchingVictory = true
			}
		}
		if !matchingVictory {
			return errors.Errorf("任务挑战敌群必须匹配本步骤的battleVictory条件: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
		}
	}
	seenItemIDs := make(map[uint32]struct{}, len(step.ConsumeItems))
	for itemIndex := range step.ConsumeItems {
		item := &step.ConsumeItems[itemIndex]
		if item.ItemID == nil || (!isItemID(*item.ItemID) && !isEquipmentID(*item.ItemID)) || item.Quantity == nil || *item.Quantity == 0 {
			return errors.Errorf("任务步骤扣除道具无效: task:%d step:%d index:%d %v", taskID, expectedStepID, itemIndex, xruntime.Location())
		}
		if _, exists := seenItemIDs[*item.ItemID]; exists {
			return errors.Errorf("任务步骤扣除道具ID重复: task:%d step:%d item:%d %v", taskID, expectedStepID, *item.ItemID, xruntime.Location())
		}
		seenItemIDs[*item.ItemID] = struct{}{}
	}
	seenPetIDs := make(map[uint32]struct{}, len(step.ConsumePets))
	for petIndex := range step.ConsumePets {
		pet := &step.ConsumePets[petIndex]
		if pet.PetID == nil || !isPetID(*pet.PetID) || pet.Level == nil ||
			*pet.Level < uint32(pb.Constants_Constants_Level_Min) || *pet.Level > uint32(pb.Constants_Constants_Level_Max) ||
			pet.Quantity == nil || *pet.Quantity == 0 {
			return errors.Errorf("任务步骤扣除宠物无效: task:%d step:%d index:%d %v", taskID, expectedStepID, petIndex, xruntime.Location())
		}
		if _, exists := seenPetIDs[*pet.PetID]; exists {
			return errors.Errorf("任务步骤扣除宠物ID重复: task:%d step:%d pet:%d %v", taskID, expectedStepID, *pet.PetID, xruntime.Location())
		}
		seenPetIDs[*pet.PetID] = struct{}{}
	}
	return nil
}

func configureTaskInteractions(taskID uint32, stepID uint32, interactions []TaskInteractionEntry) error {
	seenIDs := make(map[string]struct{}, len(interactions))
	for interactionIndex := range interactions {
		interaction := &interactions[interactionIndex]
		if interaction.ID == nil || strings.TrimSpace(*interaction.ID) == "" || interaction.Label == nil || strings.TrimSpace(*interaction.Label) == "" ||
			interaction.State == nil || *interaction.State != TaskInteractionStateUnavailable || interaction.UnavailableMessage == nil || strings.TrimSpace(*interaction.UnavailableMessage) == "" {
			return errors.Errorf("任务交互配置无效: task:%d step:%d index:%d %v", taskID, stepID, interactionIndex, xruntime.Location())
		}
		if *interaction.ID != TaskInteractionCharacterRebirth {
			return errors.Errorf("任务交互ID无效: task:%d step:%d id:%q %v", taskID, stepID, *interaction.ID, xruntime.Location())
		}
		if _, exists := seenIDs[*interaction.ID]; exists {
			return errors.Errorf("任务交互ID重复: task:%d step:%d id:%q %v", taskID, stepID, *interaction.ID, xruntime.Location())
		}
		seenIDs[*interaction.ID] = struct{}{}
		seenRebirthCounts := make(map[uint32]struct{}, len(interaction.RequirementsByRebirth))
		for requirementIndex := range interaction.RequirementsByRebirth {
			requirement := &interaction.RequirementsByRebirth[requirementIndex]
			if requirement.RebirthCount == nil || *requirement.RebirthCount > 4 || len(requirement.Pets) == 0 {
				return errors.Errorf("任务转生需求无效: task:%d step:%d index:%d %v", taskID, stepID, requirementIndex, xruntime.Location())
			}
			if _, exists := seenRebirthCounts[*requirement.RebirthCount]; exists {
				return errors.Errorf("任务转生次数需求重复: task:%d step:%d rebirth:%d %v", taskID, stepID, *requirement.RebirthCount, xruntime.Location())
			}
			seenRebirthCounts[*requirement.RebirthCount] = struct{}{}
			seenPetIDs := make(map[uint32]struct{}, len(requirement.Pets))
			for petIndex := range requirement.Pets {
				pet := &requirement.Pets[petIndex]
				if pet.PetID == nil || !isPetID(*pet.PetID) || pet.Level == nil || *pet.Level < uint32(pb.Constants_Constants_Level_Min) || *pet.Level > uint32(pb.Constants_Constants_Level_Max) || pet.Quantity == nil || *pet.Quantity == 0 {
					return errors.Errorf("任务转生宠物需求无效: task:%d step:%d rebirth:%d petIndex:%d %v", taskID, stepID, *requirement.RebirthCount, petIndex, xruntime.Location())
				}
				if _, exists := seenPetIDs[*pet.PetID]; exists {
					return errors.Errorf("任务转生宠物需求重复: task:%d step:%d rebirth:%d pet:%d %v", taskID, stepID, *requirement.RebirthCount, *pet.PetID, xruntime.Location())
				}
				seenPetIDs[*pet.PetID] = struct{}{}
			}
		}
		if len(seenRebirthCounts) != 5 {
			return errors.Errorf("任务转生需求必须完整覆盖0至4转: task:%d step:%d %v", taskID, stepID, xruntime.Location())
		}
	}
	return nil
}

func (p *TaskConfig) check() error {
	var checkErr error
	p.Foreach(func(taskID uint32, task *TaskEntry) bool {
		if err := p.checkConditions(taskID, 0, "acceptConditions", task.AcceptConditions, false); err != nil {
			checkErr = err
			return false
		}
		for _, step := range task.Steps {
			stepID := *step.ID
			if step.Navigation != nil && step.Navigation.X != nil {
				if GGameConfig.Scene == nil || GGameConfig.Scene.Get(*step.Navigation.MapID) == nil {
					checkErr = errors.Errorf("任务地图入口引用了未发布地图: task:%d step:%d map:%d %v", taskID, stepID, *step.Navigation.MapID, xruntime.Location())
					return false
				}
				if !GGameConfig.Scene.Get(*step.Navigation.MapID).IsCoordinateValid(*step.Navigation.X, *step.Navigation.Y) {
					checkErr = errors.Errorf("任务地图入口坐标越界: task:%d step:%d %v", taskID, stepID, xruntime.Location())
					return false
				}
			}
			if step.NPC != nil && step.NPC.Visual != nil && step.NPC.Visual.Type != nil && *step.NPC.Visual.Type == TaskNPCVisualTypeCharacterMount {
				visual := step.NPC.Visual
				if GGameConfig == nil || GGameConfig.Character == nil || GGameConfig.Pet == nil {
					checkErr = errors.Errorf("任务骑乘NPC缺少角色或宠物配置依赖: task:%d step:%d %v", taskID, stepID, xruntime.Location())
					return false
				}
				character := GGameConfig.Character.Get(*visual.CharacterID)
				if character == nil || GGameConfig.Pet.Get(*visual.PetID) == nil || !character.CanMount(*visual.PetID) {
					checkErr = errors.Errorf("任务骑乘NPC引用未发布或角色不可骑乘该宠物: task:%d step:%d character:%d pet:%d %v", taskID, stepID, *visual.CharacterID, *visual.PetID, xruntime.Location())
					return false
				}
			}
			if err := p.checkConditions(taskID, stepID, "startConditions", step.StartConditions, false); err != nil {
				checkErr = err
				return false
			}
			if err := p.checkConditions(taskID, stepID, "completionConditions", step.CompletionConditions, true); err != nil {
				checkErr = err
				return false
			}
			if *step.CompletionMode == TaskCompletionModeSubmit {
				for _, condition := range step.CompletionConditions {
					if condition.Kind != nil && *condition.Kind == TaskConditionKindBattleVictory {
						checkErr = errors.Errorf("主动提交步骤不能使用瞬时战斗胜利条件: task:%d step:%d %v", taskID, stepID, xruntime.Location())
						return false
					}
				}
			}
			for _, item := range step.ConsumeItems {
				if GGameConfig.Item == nil || GGameConfig.Item.Get(*item.ItemID) == nil {
					checkErr = errors.Errorf("任务步骤引用了未定义扣除道具: task:%d step:%d item:%d %v", taskID, stepID, *item.ItemID, xruntime.Location())
					return false
				}
			}
			for _, pet := range step.ConsumePets {
				if GGameConfig.Pet == nil || GGameConfig.Pet.Get(*pet.PetID) == nil {
					checkErr = errors.Errorf("任务步骤引用了未定义扣除宠物: task:%d step:%d pet:%d %v", taskID, stepID, *pet.PetID, xruntime.Location())
					return false
				}
			}
			for _, interaction := range step.Interactions {
				for _, requirement := range interaction.RequirementsByRebirth {
					for _, pet := range requirement.Pets {
						if GGameConfig.Pet == nil || GGameConfig.Pet.Get(*pet.PetID) == nil {
							checkErr = errors.Errorf("任务转生需求引用了未定义宠物: task:%d step:%d pet:%d %v", taskID, stepID, *pet.PetID, xruntime.Location())
							return false
						}
					}
				}
			}
			if *step.RewardID != 0 && (GGameConfig.Reward == nil || GGameConfig.Reward.Get(*step.RewardID) == nil) {
				checkErr = errors.Errorf("任务步骤引用了未定义奖励包: task:%d step:%d reward:%d %v", taskID, stepID, *step.RewardID, xruntime.Location())
				return false
			}
			if step.RewardReissue != nil {
				itemID := *step.RewardReissue.WhenItemAbsent
				reward := GGameConfig.Reward.Get(*step.RewardID)
				if GGameConfig.Item == nil || GGameConfig.Item.Get(itemID) == nil {
					checkErr = errors.Errorf("任务步骤补领引用了未定义道具: task:%d step:%d item:%d %v", taskID, stepID, itemID, xruntime.Location())
					return false
				}
				if reward == nil || reward.Mode == nil || *reward.Mode != RewardModeAll || len(reward.Items) != 1 || len(reward.Pets) != 0 ||
					reward.Items[0].ItemID == nil || *reward.Items[0].ItemID != itemID {
					checkErr = errors.Errorf("可补领奖励包必须只包含指定道具: task:%d step:%d item:%d %v", taskID, stepID, itemID, xruntime.Location())
					return false
				}
			}
		}
		return true
	})
	return checkErr
}

func (p *TaskConfig) checkConditions(taskID uint32, stepID uint32, field string, conditions []TaskConditionEntry, allowBattleVictory bool) error {
	for index := range conditions {
		condition := &conditions[index]
		if condition.Kind == nil {
			return errors.Errorf("任务条件缺少kind: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
		}
		switch *condition.Kind {
		case TaskConditionKindCharacterLevel:
			if condition.Level == nil || *condition.Level < uint32(pb.Constants_Constants_Level_Min) || *condition.Level > uint32(pb.Constants_Constants_Level_Max) {
				return errors.Errorf("任务角色等级条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindItemPossession:
			if condition.ItemID == nil || condition.Quantity == nil || *condition.Quantity == 0 || GGameConfig.Item == nil || GGameConfig.Item.Get(*condition.ItemID) == nil {
				return errors.Errorf("任务持有道具条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindPetPossession:
			if condition.PetID == nil || condition.Level == nil || condition.Quantity == nil || *condition.Quantity == 0 ||
				*condition.Level < uint32(pb.Constants_Constants_Level_Min) || *condition.Level > uint32(pb.Constants_Constants_Level_Max) ||
				GGameConfig.Pet == nil || GGameConfig.Pet.Get(*condition.PetID) == nil {
				return errors.Errorf("任务持有宠物条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindTaskCompleted, TaskConditionKindTaskRewardsClaimed:
			if condition.TaskID == nil || *condition.TaskID == 0 || *condition.TaskID == taskID || p.Get(*condition.TaskID) == nil {
				return errors.Errorf("任务前置条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindAnyTaskRewardsClaimed:
			if len(condition.TaskIDs) == 0 {
				return errors.Errorf("任一任务奖励前置条件不能为空: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
			seenTaskIDs := make(map[uint32]struct{}, len(condition.TaskIDs))
			for _, requiredTaskID := range condition.TaskIDs {
				if requiredTaskID == 0 || requiredTaskID == taskID || p.Get(requiredTaskID) == nil {
					return errors.Errorf("任一任务奖励前置条件无效: task:%d step:%d field:%s index:%d required:%d %v", taskID, stepID, field, index, requiredTaskID, xruntime.Location())
				}
				if _, exists := seenTaskIDs[requiredTaskID]; exists {
					return errors.Errorf("任一任务奖励前置条件包含重复任务: task:%d step:%d field:%s index:%d required:%d %v", taskID, stepID, field, index, requiredTaskID, xruntime.Location())
				}
				seenTaskIDs[requiredTaskID] = struct{}{}
			}
		case TaskConditionKindBattleVictory:
			if !allowBattleVictory || condition.EnemyGroupID == nil || *condition.EnemyGroupID == 0 || GGameConfig.Enemy == nil || GGameConfig.Enemy.Get(*condition.EnemyGroupID) == nil {
				return errors.Errorf("任务战斗胜利条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		default:
			return errors.Errorf("任务条件kind无效: task:%d step:%d field:%s index:%d kind:%q %v", taskID, stepID, field, index, *condition.Kind, xruntime.Location())
		}
		if err := validateTaskConditionFields(condition); err != nil {
			return errors.Errorf("任务条件字段无效: task:%d step:%d field:%s index:%d err:%v %v", taskID, stepID, field, index, err, xruntime.Location())
		}
	}
	return nil
}

func validateTaskConditionFields(condition *TaskConditionEntry) error {
	if condition == nil || condition.Kind == nil {
		return errors.New("condition or kind is nil")
	}
	allowed := map[string]bool{"kind": true}
	switch *condition.Kind {
	case TaskConditionKindCharacterLevel:
		allowed["level"] = true
	case TaskConditionKindItemPossession:
		allowed["itemId"] = true
		allowed["quantity"] = true
	case TaskConditionKindPetPossession:
		allowed["petId"] = true
		allowed["level"] = true
		allowed["quantity"] = true
	case TaskConditionKindTaskCompleted, TaskConditionKindTaskRewardsClaimed:
		allowed["taskId"] = true
	case TaskConditionKindAnyTaskRewardsClaimed:
		allowed["taskIds"] = true
	case TaskConditionKindBattleVictory:
		allowed["enemyGroupId"] = true
	}
	if condition.Level != nil && !allowed["level"] {
		return errors.New("unexpected level")
	}
	if condition.ItemID != nil && !allowed["itemId"] {
		return errors.New("unexpected itemId")
	}
	if condition.PetID != nil && !allowed["petId"] {
		return errors.New("unexpected petId")
	}
	if condition.Quantity != nil && !allowed["quantity"] {
		return errors.New("unexpected quantity")
	}
	if condition.TaskID != nil && !allowed["taskId"] {
		return errors.New("unexpected taskId")
	}
	if len(condition.TaskIDs) > 0 && !allowed["taskIds"] {
		return errors.New("unexpected taskIds")
	}
	if condition.EnemyGroupID != nil && !allowed["enemyGroupId"] {
		return errors.New("unexpected enemyGroupId")
	}
	return nil
}

func (p *TaskConfig) assemble() error {
	return nil
}
