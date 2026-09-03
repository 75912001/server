package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type RewardConfig struct {
	*xmap.MapMgr[uint32, *RewardEntry]
}

type RewardEntry struct {
	ID    *uint32           `yaml:"id"`
	Name  *string           `yaml:"name"`
	Items []RewardItemEntry `yaml:"items"`
}

type RewardItemEntry struct {
	ItemID   *uint32 `yaml:"itemId"`
	Quantity *uint64 `yaml:"quantity"`
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
		if len(reward.Items) == 0 {
			return errors.Errorf("奖励包道具不能为空: id:%d %v", *reward.ID, xruntime.Location())
		}
		seenItemIDs := make(map[uint32]struct{}, len(reward.Items))
		for itemIndex := range reward.Items {
			item := &reward.Items[itemIndex]
			if item.ItemID == nil || !isItemID(*item.ItemID) {
				return errors.Errorf("奖励包道具ID不能为空: reward:%d index:%d %v", *reward.ID, itemIndex, xruntime.Location())
			}
			if item.Quantity == nil || *item.Quantity == 0 {
				return errors.Errorf("奖励包道具数量必须大于0: reward:%d item:%d %v", *reward.ID, *item.ItemID, xruntime.Location())
			}
			if _, exists := seenItemIDs[*item.ItemID]; exists {
				return errors.Errorf("奖励包道具ID重复: reward:%d item:%d %v", *reward.ID, *item.ItemID, xruntime.Location())
			}
			seenItemIDs[*item.ItemID] = struct{}{}
		}
		if !p.AddIfNotExist(*reward.ID, reward) {
			return errors.Errorf("奖励包ID重复: %d %v", *reward.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *RewardConfig) check() error {
	var checkErr error
	p.Foreach(func(rewardID uint32, reward *RewardEntry) bool {
		for _, item := range reward.Items {
			if GGameConfig.Item == nil || GGameConfig.Item.Get(*item.ItemID) == nil {
				checkErr = errors.Errorf("奖励包引用了未定义道具: reward:%d item:%d %v", rewardID, *item.ItemID, xruntime.Location())
				return false
			}
		}
		return true
	})
	return checkErr
}

func (p *RewardConfig) assemble() error {
	return nil
}
