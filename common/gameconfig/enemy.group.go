package gameconfig

import (
	"math"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	pb "server/proto/pb"
)

type EnemyGroupConfig struct {
	*xmap.MapMgr[uint32, *EnemyGroupEntry]
}

type EnemyGroupEntry struct {
	// ID 来自 enemyGroups[].id, 必须为正数, 并且在 enemy.group.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 enemyGroups[].name, 用于配置审计和错误定位.
	Name *string `yaml:"name"`
	// IsBoss 来自 enemyGroups[].isBoss, 缺省为 false; Boss 组按固定 enemies 顺序出怪, 不使用普通组随机规则.
	IsBoss *bool `yaml:"isBoss"`
	// CountRange 来自 enemyGroups[].countRange, 表示普通敌人组出怪数量范围; Boss 组不允许配置.
	CountRange *IntRange `yaml:"countRange"`
	// LevelRange 来自 enemyGroups[].levelRange, 表示普通敌人组随机等级范围; 与 RoleLevelOffset 必须且只能配置一个, Boss 组不允许配置.
	LevelRange *IntRange `yaml:"levelRange"`
	// RoleLevelOffset 来自 enemyGroups[].roleLevelOffset, 表示基于玩家等级的随机偏移范围; 与 LevelRange 必须且只能配置一个, Boss 组不允许配置.
	RoleLevelOffset *IntRange `yaml:"roleLevelOffset"`
	// Captured 来自 enemyGroups[].captured, 表示普通敌人组是否允许捕获, 缺省为 true; Boss 组固定为 false.
	Captured *bool `yaml:"captured"`
	// BabyRate 来自 enemyGroups[].babyRate, 表示每只敌人成为 1 级宠物宝宝的十万分率, 缺省为0; Boss 组不允许配置.
	BabyRate *uint32 `yaml:"babyRate"`
	// Enemies 来自 enemyGroups[].enemies, 保存敌人模板列表, 每个敌人ID必须引用 pet.yaml 中存在的宠物ID.
	Enemies []EnemyEntry `yaml:"enemies"`
}

type EnemyEntry struct {
	// ID 来自 enemies[].id, 表示作为敌人模板的宠物ID, 必须能在 pet.yaml 中找到.
	ID *uint32 `yaml:"id"`
	// Weight 来自 enemies[].weight, 表示普通敌人组随机选择权重, 缺省为0且代表必定出现; Boss 组不允许配置.
	Weight *uint32 `yaml:"weight"`
	// Level 来自 enemies[].level, 表示指定敌人等级; Boss 组必填, 普通组可选, 值必须处于协议等级范围.
	Level *uint32 `yaml:"level"`
}

type IntRange struct {
	// Min 表示闭区间最小值, 由 YAML 中二元数组的第一个元素解析得到.
	Min *int
	// Max 表示闭区间最大值, 由 YAML 中二元数组的第二个元素解析得到, 且必须大于等于 Min.
	Max *int
}

// UnmarshalYAML 严格读取 YAML 二元整数数组, 并保证最小值不大于最大值.
func (p *IntRange) UnmarshalYAML(node *yaml.Node) error {
	var values []int
	if err := node.Decode(&values); err != nil {
		return err
	}
	if len(values) != 2 {
		return errors.Errorf("整数范围必须包含2个值: got:%d", len(values))
	}
	if values[0] > values[1] {
		return errors.Errorf("整数范围最小值不能大于最大值: min:%d max:%d", values[0], values[1])
	}

	p.Min = valuePtr(values[0])
	p.Max = valuePtr(values[1])
	return nil
}

func newEnemyGroupConfig() *EnemyGroupConfig {
	return &EnemyGroupConfig{
		MapMgr: xmap.NewMapMgr[uint32, *EnemyGroupEntry](),
	}
}

func (p *EnemyGroupConfig) load(dir string) error {
	var root struct {
		EnemyGroups []*EnemyGroupEntry `yaml:"enemyGroups"`
	}
	if err := loadYAMLFile(dir, FileEnemyGroup, &root); err != nil {
		return err
	}
	return p.configure(root.EnemyGroups)
}

func (p *EnemyGroupConfig) configure(entries []*EnemyGroupEntry) error {
	for i, group := range entries {
		if group == nil {
			return errors.Errorf("敌人组不能为空: index:%d %v", i, xruntime.Location())
		}
		if group.ID == nil {
			return errors.Errorf("敌人组缺少 id: index:%d %v", i, xruntime.Location())
		}
		if *group.ID == 0 {
			return errors.Errorf("敌人组ID非法: id:%d %v", *group.ID, xruntime.Location())
		}

		if group.IsBoss == nil {
			defaultValue := false
			group.IsBoss = &defaultValue
		}

		if *group.IsBoss {
			if group.CountRange != nil {
				return errors.Errorf("Boss 敌人组不允许配置 countRange: group:%d %v", *group.ID, xruntime.Location())
			}
			if group.LevelRange != nil {
				return errors.Errorf("Boss 敌人组不允许配置 levelRange: group:%d %v", *group.ID, xruntime.Location())
			}
			if group.RoleLevelOffset != nil {
				return errors.Errorf("Boss 敌人组不允许配置 roleLevelOffset: group:%d %v", *group.ID, xruntime.Location())
			}
			if group.Captured != nil {
				return errors.Errorf("Boss 敌人组不允许配置 captured: group:%d %v", *group.ID, xruntime.Location())
			}
			if group.BabyRate != nil {
				return errors.Errorf("Boss 敌人组不允许配置 babyRate: group:%d %v", *group.ID, xruntime.Location())
			}
			captured := false
			group.Captured = &captured
			babyRate := uint32(0)
			group.BabyRate = &babyRate
		} else {
			if group.CountRange == nil || group.CountRange.Min == nil || group.CountRange.Max == nil {
				return errors.Errorf("普通敌人组缺少有效 countRange: group:%d %v", *group.ID, xruntime.Location())
			}
			if *group.CountRange.Min < int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Min) ||
				*group.CountRange.Max > int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Max) {
				return errors.Errorf("普通敌人组 countRange 超出范围: group:%d range:[%d,%d] expected:[%d,%d] %v",
					*group.ID, *group.CountRange.Min, *group.CountRange.Max,
					pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Min,
					pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Max, xruntime.Location())
			}
			if (group.LevelRange == nil) == (group.RoleLevelOffset == nil) {
				return errors.Errorf("普通敌人组 levelRange 和 roleLevelOffset 必须且只能配置一个: group:%d %v",
					*group.ID, xruntime.Location())
			}
			if group.LevelRange != nil {
				if group.LevelRange.Min == nil || group.LevelRange.Max == nil ||
					*group.LevelRange.Min < int(pb.LevelRange_LevelRange_Min) ||
					*group.LevelRange.Max > int(pb.LevelRange_LevelRange_Max) {
					return errors.Errorf("普通敌人组 levelRange 超出范围: group:%d %v", *group.ID, xruntime.Location())
				}
			}
			if group.RoleLevelOffset != nil &&
				(group.RoleLevelOffset.Min == nil || group.RoleLevelOffset.Max == nil) {
				return errors.Errorf("普通敌人组 roleLevelOffset 无效: group:%d %v", *group.ID, xruntime.Location())
			}
			if group.Captured == nil {
				defaultValue := true
				group.Captured = &defaultValue
			}
			if group.BabyRate == nil {
				defaultValue := uint32(0)
				group.BabyRate = &defaultValue
			}
			if *group.BabyRate < uint32(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Min) || *group.BabyRate > uint32(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Max) {
				return errors.Errorf("敌人组 babyRate 超出范围: group:%d value:%d %v", *group.ID, *group.BabyRate, xruntime.Location())
			}
		}

		if len(group.Enemies) == 0 {
			return errors.Errorf("敌人组 enemies 不能为空: group:%d %v", *group.ID, xruntime.Location())
		}
		if len(group.Enemies) > int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Max) {
			return errors.Errorf("敌人组 enemies 超过最大站位数量: group:%d size:%d %v", *group.ID, len(group.Enemies), xruntime.Location())
		}
		for enemyIndex := range group.Enemies {
			enemy := &group.Enemies[enemyIndex]
			if enemy.ID == nil {
				return errors.Errorf("敌人组 enemy 缺少 id: group:%d index:%d %v", *group.ID, enemyIndex, xruntime.Location())
			}
			enemyID := *enemy.ID
			if *group.IsBoss {
				if enemy.Weight != nil {
					return errors.Errorf("Boss 敌人组不允许配置 weight: group:%d enemy:%d %v", *group.ID, enemyID, xruntime.Location())
				}
				if enemy.Level == nil {
					return errors.Errorf("Boss 敌人组必须配置 level: group:%d enemy:%d %v", *group.ID, enemyID, xruntime.Location())
				}
				if *enemy.Level < uint32(pb.LevelRange_LevelRange_Min) ||
					uint32(pb.LevelRange_LevelRange_Max) < *enemy.Level {
					return errors.Errorf("Boss 敌人组 enemy level 超出范围: group:%d enemy:%d level:%d %v", *group.ID, enemyID, *enemy.Level, xruntime.Location())
				}
				continue
			}

			if enemy.Weight == nil {
				weight := uint32(0)
				enemy.Weight = &weight
			}
			if *enemy.Weight > uint32(math.MaxInt32) {
				return errors.Errorf("普通敌人组条目weight超出C int范围: group:%d enemy:%d weight:%d %v",
					*group.ID, enemyID, *enemy.Weight, xruntime.Location())
			}
			if enemy.Level != nil &&
				(*enemy.Level < uint32(pb.LevelRange_LevelRange_Min) ||
					uint32(pb.LevelRange_LevelRange_Max) < *enemy.Level) {
				return errors.Errorf("敌人组 enemy level 超出范围: group:%d enemy:%d level:%d %v",
					*group.ID, enemyID, *enemy.Level, xruntime.Location())
			}
		}
		if !*group.IsBoss {
			requiredCount := 0
			totalWeight := uint64(0)
			for enemyIndex := range group.Enemies {
				weight := *group.Enemies[enemyIndex].Weight
				if weight == 0 {
					requiredCount++
					continue
				}
				totalWeight += uint64(weight)
			}
			if requiredCount > *group.CountRange.Min {
				return errors.Errorf("普通敌人组必出敌人数超过countRange下限: group:%d required:%d min:%d %v",
					*group.ID, requiredCount, *group.CountRange.Min, xruntime.Location())
			}
			if requiredCount < *group.CountRange.Max && totalWeight == 0 {
				return errors.Errorf("普通敌人组缺少可填充countRange的正权重敌人: group:%d %v",
					*group.ID, xruntime.Location())
			}
			if totalWeight > uint64(math.MaxInt32) {
				return errors.Errorf("普通敌人组总权重超出C int范围: group:%d total:%d %v",
					*group.ID, totalWeight, xruntime.Location())
			}
		}

		if !p.AddIfNotExist(*group.ID, group) {
			return errors.Errorf("敌人组ID重复: %d %v", *group.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *EnemyGroupConfig) check() error {
	var checkErr error
	p.Foreach(func(_ uint32, group *EnemyGroupEntry) bool {
		for _, enemy := range group.Enemies {
			if !GGameConfig.Pet.IsExist(*enemy.ID) {
				checkErr = errors.Errorf("敌人组引用了未定义宠物: group:%d pet:%d %v",
					*group.ID, *enemy.ID, xruntime.Location())
				return false
			}
		}
		return true
	})
	return checkErr
}

func (p *EnemyGroupConfig) assemble() error {
	return nil
}
