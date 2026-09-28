package gameconfig

import (
	"math"
	"strings"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type RewardConfig struct {
	*xmap.MapMgr[uint32, *RewardEntry]
}

type RewardMode string

const (
	RewardModeAll       RewardMode = "all"
	RewardModeRandomOne RewardMode = "randomOne"
)

type RewardEntry struct {
	ID         *uint32                `yaml:"id"`
	Name       *string                `yaml:"name"`
	Mode       *RewardMode            `yaml:"mode"`
	Items      []RewardItemEntry      `yaml:"items"`
	Pets       []RewardPetEntry       `yaml:"pets"`
	Candidates []RewardCandidateEntry `yaml:"candidates"`
}

// RewardCandidateEntry 是 randomOne 奖励包中的一个加权候选组.
// 抽中后组内的全部道具和宠物作为一个不可拆分的整体发放.
type RewardCandidateEntry struct {
	Weight *uint64           `yaml:"weight"`
	Items  []RewardItemEntry `yaml:"items"`
	Pets   []RewardPetEntry  `yaml:"pets"`
}

type RewardItemEntry struct {
	ItemID   *uint32 `yaml:"itemId"`
	Quantity *uint64 `yaml:"quantity"`
}

type RewardPetGrade string

const RewardPetGradeRandom RewardPetGrade = "random"

type RewardPetEntry struct {
	PetID    *uint32         `yaml:"petId"`
	Level    *uint32         `yaml:"level"`
	Grade    *RewardPetGrade `yaml:"grade"`
	Quantity *uint32         `yaml:"quantity"`
}

func newRewardConfig() *RewardConfig {
	return &RewardConfig{MapMgr: xmap.NewMapMgr[uint32, *RewardEntry]()}
}

func (p *RewardConfig) load(dir string) error {
	var root struct {
		Rewards []*RewardEntry `yaml:"rewards"`
	}
	if err := loadYAMLFile(dir, FileReward, &root); err != nil {
		return err
	}
	return p.configure(root.Rewards)
}

func (p *RewardConfig) configure(entries []*RewardEntry) error {
	for index, reward := range entries {
		if reward == nil {
			return errors.Errorf("奖励包不能为空: index:%d %v", index, xruntime.Location())
		}
		if reward.ID == nil || *reward.ID == 0 {
			return errors.Errorf("奖励包ID必须大于0: index:%d %v", index, xruntime.Location())
		}
		if reward.Name == nil || strings.TrimSpace(*reward.Name) == "" {
			return errors.Errorf("奖励包名称不能为空: id:%d %v", *reward.ID, xruntime.Location())
		}
		if reward.Mode == nil {
			return errors.Errorf("奖励包模式不能为空: id:%d %v", *reward.ID, xruntime.Location())
		}
		switch *reward.Mode {
		case RewardModeAll:
			if len(reward.Candidates) != 0 {
				return errors.Errorf("全部发放奖励包不能配置候选组: id:%d %v", *reward.ID, xruntime.Location())
			}
			if err := validateRewardContents(*reward.ID, -1, reward.Items, reward.Pets); err != nil {
				return err
			}
		case RewardModeRandomOne:
			if len(reward.Items) != 0 || len(reward.Pets) != 0 || len(reward.Candidates) == 0 {
				return errors.Errorf("随机奖励包必须只配置非空候选组: id:%d %v", *reward.ID, xruntime.Location())
			}
			var totalWeight uint64
			for candidateIndex := range reward.Candidates {
				candidate := &reward.Candidates[candidateIndex]
				if candidate.Weight == nil || *candidate.Weight == 0 {
					return errors.Errorf("随机奖励候选权重必须大于0: reward:%d candidate:%d %v", *reward.ID, candidateIndex, xruntime.Location())
				}
				if totalWeight > math.MaxUint64-*candidate.Weight {
					return errors.Errorf("随机奖励候选总权重溢出: reward:%d %v", *reward.ID, xruntime.Location())
				}
				totalWeight += *candidate.Weight
				if err := validateRewardContents(*reward.ID, candidateIndex, candidate.Items, candidate.Pets); err != nil {
					return err
				}
			}
		default:
			return errors.Errorf("奖励包模式无效: id:%d mode:%q %v", *reward.ID, *reward.Mode, xruntime.Location())
		}
		if !p.AddIfNotExist(*reward.ID, reward) {
			return errors.Errorf("奖励包ID重复: %d %v", *reward.ID, xruntime.Location())
		}
	}
	return nil
}

func validateRewardContents(rewardID uint32, candidateIndex int, items []RewardItemEntry, pets []RewardPetEntry) error {
	context := errors.Errorf("reward:%d", rewardID)
	if candidateIndex >= 0 {
		context = errors.Errorf("reward:%d candidate:%d", rewardID, candidateIndex)
	}
	if len(items) == 0 && len(pets) == 0 {
		return errors.Errorf("奖励内容不能为空: %v %v", context, xruntime.Location())
	}
	seenItemIDs := make(map[uint32]struct{}, len(items))
	for itemIndex := range items {
		item := &items[itemIndex]
		if item.ItemID == nil || (!isItemID(*item.ItemID) && !isEquipmentID(*item.ItemID)) {
			return errors.Errorf("奖励道具ID无效: %v index:%d %v", context, itemIndex, xruntime.Location())
		}
		if item.Quantity == nil || *item.Quantity == 0 {
			return errors.Errorf("奖励道具数量必须大于0: %v item:%d %v", context, *item.ItemID, xruntime.Location())
		}
		if _, exists := seenItemIDs[*item.ItemID]; exists {
			return errors.Errorf("奖励道具ID重复: %v item:%d %v", context, *item.ItemID, xruntime.Location())
		}
		seenItemIDs[*item.ItemID] = struct{}{}
	}
	seenPetIDs := make(map[uint32]struct{}, len(pets))
	for petIndex := range pets {
		pet := &pets[petIndex]
		if pet.PetID == nil || !isPetID(*pet.PetID) {
			return errors.Errorf("奖励宠物ID无效: %v index:%d %v", context, petIndex, xruntime.Location())
		}
		if pet.Level == nil || *pet.Level < uint32(pb.Constants_Constants_Level_Min) || *pet.Level > uint32(pb.Constants_Constants_Level_Max) {
			return errors.Errorf("奖励宠物等级无效: %v pet:%d %v", context, *pet.PetID, xruntime.Location())
		}
		if pet.Grade == nil || *pet.Grade != RewardPetGradeRandom {
			return errors.Errorf("奖励宠物品质无效: %v pet:%d %v", context, *pet.PetID, xruntime.Location())
		}
		if pet.Quantity == nil || *pet.Quantity == 0 {
			return errors.Errorf("奖励宠物数量必须大于0: %v pet:%d %v", context, *pet.PetID, xruntime.Location())
		}
		if _, exists := seenPetIDs[*pet.PetID]; exists {
			return errors.Errorf("奖励宠物ID重复: %v pet:%d %v", context, *pet.PetID, xruntime.Location())
		}
		seenPetIDs[*pet.PetID] = struct{}{}
	}
	return nil
}

func (p *RewardConfig) check() error {
	var checkErr error
	p.Foreach(func(rewardID uint32, reward *RewardEntry) bool {
		contents := []RewardCandidateEntry{{Items: reward.Items, Pets: reward.Pets}}
		if reward.Mode != nil && *reward.Mode == RewardModeRandomOne {
			contents = reward.Candidates
		}
		for _, content := range contents {
			for _, item := range content.Items {
				if GGameConfig.Item == nil || GGameConfig.Item.Get(*item.ItemID) == nil {
					checkErr = errors.Errorf("奖励包引用了未定义道具: reward:%d item:%d %v", rewardID, *item.ItemID, xruntime.Location())
					return false
				}
			}
			for _, pet := range content.Pets {
				if GGameConfig.Pet == nil || GGameConfig.Pet.Get(*pet.PetID) == nil {
					checkErr = errors.Errorf("奖励包引用了未定义宠物: reward:%d pet:%d %v", rewardID, *pet.PetID, xruntime.Location())
					return false
				}
			}
		}
		return true
	})
	return checkErr
}

func (p *RewardConfig) assemble() error {
	return nil
}
