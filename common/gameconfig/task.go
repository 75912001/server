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
	TaskCompletionModeAutomatic TaskCompletionMode = "automatic"
	TaskCompletionModeSubmit    TaskCompletionMode = "submit"
)

type TaskConditionKind string

const (
	TaskConditionKindCharacterLevel TaskConditionKind = "characterLevel"
	TaskConditionKindItemPossession TaskConditionKind = "itemPossession"
	TaskConditionKindTaskCompleted  TaskConditionKind = "taskCompleted"
	TaskConditionKindBattleVictory  TaskConditionKind = "battleVictory"
)

type TaskConfig struct {
	*xmap.MapMgr[uint32, *TaskEntry]
}

type TaskEntry struct {
	ID               *uint32              `yaml:"id"`
	Name             *string              `yaml:"name"`
	Description      *string              `yaml:"description"`
	IsMain           *bool                `yaml:"isMain"`
	Sort             int32                `yaml:"sort"`
	AcceptConditions []TaskConditionEntry `yaml:"acceptConditions"`
	Steps            []*TaskStepEntry     `yaml:"steps"`
}

type TaskStepEntry struct {
	ID                   *uint32              `yaml:"id"`
	Name                 *string              `yaml:"name"`
	Description          *string              `yaml:"description"`
	StartConditions      []TaskConditionEntry `yaml:"startConditions"`
	CompletionConditions []TaskConditionEntry `yaml:"completionConditions"`
	CompletionMode       *TaskCompletionMode  `yaml:"completionMode"`
	Challenge            *TaskChallengeEntry  `yaml:"challenge"`
	ConsumeItems         []TaskItemEntry      `yaml:"consumeItems"`
	RewardID             *uint32              `yaml:"rewardId"`
}

// TaskChallengeEntry只读取服务端开战所需的敌群. 同一challenge下的
// npcPetIds和battleBgmIndex由客户端读取, 不参与服务端战斗和任务判定.
type TaskChallengeEntry struct {
	EnemyGroupID *uint32 `yaml:"enemyGroupId"`
}

type TaskConditionEntry struct {
	Kind         *TaskConditionKind `yaml:"kind"`
	Level        *uint32            `yaml:"level"`
	ItemID       *uint32            `yaml:"itemId"`
	Quantity     *uint64            `yaml:"quantity"`
	TaskID       *uint32            `yaml:"taskId"`
	EnemyGroupID *uint32            `yaml:"enemyGroupId"`
}

type TaskItemEntry struct {
	ItemID   *uint32 `yaml:"itemId"`
	Quantity *uint64 `yaml:"quantity"`
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
		if len(task.Steps) == 0 {
			return errors.Errorf("任务步骤不能为空: id:%d %v", *task.ID, xruntime.Location())
		}
		for stepIndex, step := range task.Steps {
			if err := configureTaskStep(*task.ID, uint32(stepIndex+1), step); err != nil {
				return err
			}
		}
		if !p.AddIfNotExist(*task.ID, task) {
			return errors.Errorf("任务ID重复: %d %v", *task.ID, xruntime.Location())
		}
	}
	return nil
}

func configureTaskStep(taskID uint32, expectedStepID uint32, step *TaskStepEntry) error {
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
	if len(step.CompletionConditions) == 0 {
		return errors.Errorf("任务步骤完成条件不能为空: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if step.CompletionMode == nil {
		step.CompletionMode = valuePtr(TaskCompletionModeAutomatic)
	}
	if *step.CompletionMode != TaskCompletionModeAutomatic && *step.CompletionMode != TaskCompletionModeSubmit {
		return errors.Errorf("任务步骤completionMode无效: task:%d step:%d mode:%q %v", taskID, expectedStepID, *step.CompletionMode, xruntime.Location())
	}
	if step.RewardID == nil {
		return errors.Errorf("任务步骤缺少rewardId: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
	}
	if len(step.ConsumeItems) > 0 && *step.CompletionMode != TaskCompletionModeSubmit {
		return errors.Errorf("配置consumeItems的任务步骤必须主动提交: task:%d step:%d %v", taskID, expectedStepID, xruntime.Location())
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
		if item.ItemID == nil || !isItemID(*item.ItemID) || item.Quantity == nil || *item.Quantity == 0 {
			return errors.Errorf("任务步骤扣除道具无效: task:%d step:%d index:%d %v", taskID, expectedStepID, itemIndex, xruntime.Location())
		}
		if _, exists := seenItemIDs[*item.ItemID]; exists {
			return errors.Errorf("任务步骤扣除道具ID重复: task:%d step:%d item:%d %v", taskID, expectedStepID, *item.ItemID, xruntime.Location())
		}
		seenItemIDs[*item.ItemID] = struct{}{}
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
			if *step.RewardID != 0 && (GGameConfig.Reward == nil || GGameConfig.Reward.Get(*step.RewardID) == nil) {
				checkErr = errors.Errorf("任务步骤引用了未定义奖励包: task:%d step:%d reward:%d %v", taskID, stepID, *step.RewardID, xruntime.Location())
				return false
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
			if condition.Level == nil || *condition.Level < uint32(pb.LevelRange_LevelRange_Min) || *condition.Level > uint32(pb.LevelRange_LevelRange_Max) {
				return errors.Errorf("任务角色等级条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindItemPossession:
			if condition.ItemID == nil || condition.Quantity == nil || *condition.Quantity == 0 || GGameConfig.Item == nil || GGameConfig.Item.Get(*condition.ItemID) == nil {
				return errors.Errorf("任务持有道具条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
			}
		case TaskConditionKindTaskCompleted:
			if condition.TaskID == nil || *condition.TaskID == 0 || *condition.TaskID == taskID || p.Get(*condition.TaskID) == nil {
				return errors.Errorf("任务完成前置条件无效: task:%d step:%d field:%s index:%d %v", taskID, stepID, field, index, xruntime.Location())
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
	case TaskConditionKindTaskCompleted:
		allowed["taskId"] = true
	case TaskConditionKindBattleVictory:
		allowed["enemyGroupId"] = true
	}
	if condition.Level != nil && !allowed["level"] {
		return errors.New("unexpected level")
	}
	if condition.ItemID != nil && !allowed["itemId"] {
		return errors.New("unexpected itemId")
	}
	if condition.Quantity != nil && !allowed["quantity"] {
		return errors.New("unexpected quantity")
	}
	if condition.TaskID != nil && !allowed["taskId"] {
		return errors.New("unexpected taskId")
	}
	if condition.EnemyGroupID != nil && !allowed["enemyGroupId"] {
		return errors.New("unexpected enemyGroupId")
	}
	return nil
}

func (p *TaskConfig) assemble() error {
	return nil
}
