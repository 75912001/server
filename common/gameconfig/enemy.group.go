package gameconfig

import (
	"math"
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	pb "server/proto/pb"
)

type EnemyGroupConfig struct {
	*xmap.MapMgr[uint32, *EnemyGroupEntry]
}

const (
	enemyNormalDropMaxCount       = 10
	enemyNormalDropProbabilityMax = 10000
)

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
	// ID 仅保留给Go内存构造的兼容测试和旧内部调用; YAML中的id始终由UnmarshalYAML拒绝.
	ID *uint32 `yaml:"-"`
	// PetID 和 CharacterID 必须且只能配置一个, 决定敌人的外观和捕获资格.
	PetID       *uint32 `yaml:"petId"`
	CharacterID *uint32 `yaml:"characterId"`
	// GrowthAttributeID 为所有敌人成员必填, 决定四维、元素、抗性、经验和固有特性.
	GrowthAttributeID *uint32 `yaml:"growthAttributeId"`
	// GrowthAttribute 在assemble阶段挂载已校验的只读成长属性.
	GrowthAttribute *GrowthAttributeEntry `yaml:"-"`
	// Weapon 仅角色外观必填, 使用unarmed/axe/stick/spear/bow.
	Weapon *string `yaml:"weapon"`
	// WeaponType 是角色敌人的协议和战斗运行态快照, 宠物保持Unspecified.
	WeaponType pb.CharacterWeaponType `yaml:"-"`
	// DisplayName 可选覆盖敌人在战斗中的显示名称; 省略时使用外观配置名称.
	DisplayName *string `yaml:"displayName"`
	// Weight 来自 enemies[].weight, 表示普通敌人组随机选择权重, 缺省为0且代表必定出现; Boss 组不允许配置.
	Weight *uint32 `yaml:"weight"`
	// Level 来自 enemies[].level, 表示固定敌人等级; 与 LevelRange 互斥, Boss 组必须配置其中一个, 值必须处于协议等级范围.
	Level *uint32 `yaml:"level"`
	// LevelRange 来自 enemies[].levelRange, 表示本成员的随机等级闭区间; 普通组未配置成员等级时使用组级规则.
	LevelRange *IntRange `yaml:"levelRange"`
	// GradeRange 可选限制敌人实际品阶的闭区间; 省略时保持原有完全随机品阶.
	GradeRange *IntRange `yaml:"gradeRange"`
	// AttributeModifiers 可选覆盖该敌人成员相对宠物模板的八项战斗属性修正.
	AttributeModifiers *EnemyAttributeModifierEntry `yaml:"attributeModifiers"`
	// BattleAIID 来自 enemies[].battleAI, 必须显式引用ai.yaml; 敌人不单独配置技能.
	BattleAIID *uint32 `yaml:"battleAI"`
	// BattleAI 在assemble阶段挂载已校验的只读AI配置, 供建房时复制为独立快照.
	BattleAI *BattleAIEntry `yaml:"-"`
	// NormalDrops 来自 enemies[].normalDrops, 每项按万分比在敌人实例创建时独立判定一次.
	NormalDrops []EnemyNormalDropEntry `yaml:"normalDrops"`
}

// EnemyAttributeModifierEntry保存敌人成员相对宠物模板的战斗属性修正值.
// 零值表示不修正, 最终运行值不截断, 直接使用宠物原值与修正值之和.
type EnemyAttributeModifierEntry struct {
	PoisonResist    int32 `yaml:"poisonResist"`
	ParalysisResist int32 `yaml:"paralysisResist"`
	SleepResist     int32 `yaml:"sleepResist"`
	StoneResist     int32 `yaml:"stoneResist"`
	DrunkResist     int32 `yaml:"drunkResist"`
	ConfusionResist int32 `yaml:"confusionResist"`
	Critical        int32 `yaml:"critical"`
	Counter         int32 `yaml:"counter"`
}

// UnmarshalYAML拒绝拼写错误或未开放的敌人成员属性修正字段.
func (p *EnemyAttributeModifierEntry) UnmarshalYAML(node *yaml.Node) error {
	allowed := map[string]struct{}{
		"poisonResist": {}, "paralysisResist": {}, "sleepResist": {}, "stoneResist": {},
		"drunkResist": {}, "confusionResist": {}, "critical": {}, "counter": {},
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		field := node.Content[index].Value
		if _, ok := allowed[field]; !ok {
			return errors.Errorf("enemy attributeModifiers 字段未知: %s", field)
		}
	}
	type modifierEntry EnemyAttributeModifierEntry
	return node.Decode((*modifierEntry)(p))
}

// EnemyNormalDropEntry定义普通PVE敌人的一个原版掉落槽.
// 相同ItemID可以重复出现, 表示多个按配置顺序独立判定的槽位.
type EnemyNormalDropEntry struct {
	ItemID      *uint32 `yaml:"itemId"`
	Probability *uint32 `yaml:"probability"`
}

var enemyCharacterWeaponTypes = map[string]pb.CharacterWeaponType{
	"unarmed": pb.CharacterWeaponType_CharacterWeaponType_Unarmed,
	"axe":     pb.CharacterWeaponType_CharacterWeaponType_Axe,
	"stick":   pb.CharacterWeaponType_CharacterWeaponType_Stick,
	"spear":   pb.CharacterWeaponType_CharacterWeaponType_Spear,
	"bow":     pb.CharacterWeaponType_CharacterWeaponType_Bow,
}

// UnmarshalYAML拒绝旧ID和技能覆盖字段, 避免迁移遗漏被静默忽略.
func (p *EnemyEntry) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		switch node.Content[index].Value {
		case "id":
			return errors.New("enemy.group.yaml 不再允许 enemies[].id, 请使用 petId 或 characterId")
		case "skill":
			return errors.New("enemy.group.yaml 不再允许 enemies[].skill, 请在 ai.yaml 配置战斗技能")
		}
	}
	type enemyEntry EnemyEntry
	return node.Decode((*enemyEntry)(p))
}

// AssetID 返回敌人外观资源ID. 调用前必须已通过互斥校验.
func (p *EnemyEntry) AssetID() uint32 {
	if p == nil {
		return 0
	}
	if p.PetID != nil {
		return *p.PetID
	}
	if p.CharacterID != nil {
		return *p.CharacterID
	}
	if p.ID != nil {
		return *p.ID
	}
	return 0
}

func (p *EnemyEntry) IsPet() bool {
	return p != nil && (p.PetID != nil || (p.PetID == nil && p.CharacterID == nil && p.ID != nil))
}

func (p *EnemyEntry) IsCharacter() bool { return p != nil && p.CharacterID != nil }

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
					*group.LevelRange.Min < int(pb.Constants_Constants_Level_Min) ||
					*group.LevelRange.Max > int(pb.Constants_Constants_Level_Max) {
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
			if enemy.PetID == nil && enemy.CharacterID == nil && enemy.ID != nil {
				enemy.PetID = enemy.ID
			}
			if enemy.ID == nil {
				if enemy.PetID != nil {
					enemy.ID = enemy.PetID
				} else if enemy.CharacterID != nil {
					enemy.ID = enemy.CharacterID
				}
			}
			if (enemy.PetID == nil) == (enemy.CharacterID == nil) {
				return errors.Errorf("敌人组 enemy petId 和 characterId 必须且只能配置一个: group:%d index:%d %v",
					*group.ID, enemyIndex, xruntime.Location())
			}
			enemyID := enemy.AssetID()
			if enemyID == 0 || (enemy.IsPet() && !isPetID(enemyID)) || (enemy.IsCharacter() && !isCharacterID(enemyID)) {
				return errors.Errorf("敌人组 enemy 外观ID非法: group:%d enemy:%d %v", *group.ID, enemyID, xruntime.Location())
			}
			if enemy.GrowthAttributeID == nil || *enemy.GrowthAttributeID == 0 {
				return errors.Errorf("敌人组 enemy 缺少有效 growthAttributeId: group:%d enemy:%d %v",
					*group.ID, enemyID, xruntime.Location())
			}
			if enemy.IsPet() {
				if enemy.Weapon != nil {
					return errors.Errorf("宠物敌人不允许配置 weapon: group:%d pet:%d %v", *group.ID, enemyID, xruntime.Location())
				}
				enemy.WeaponType = pb.CharacterWeaponType_CharacterWeaponType_Unspecified
			} else {
				if enemy.Weapon == nil {
					return errors.Errorf("角色敌人缺少 weapon: group:%d character:%d %v", *group.ID, enemyID, xruntime.Location())
				}
				weapon := strings.TrimSpace(*enemy.Weapon)
				weaponType, exists := enemyCharacterWeaponTypes[weapon]
				if !exists {
					return errors.Errorf("角色敌人 weapon 非法: group:%d character:%d weapon:%q %v",
						*group.ID, enemyID, weapon, xruntime.Location())
				}
				enemy.Weapon = &weapon
				enemy.WeaponType = weaponType
			}
			if enemy.DisplayName != nil {
				displayName := strings.TrimSpace(*enemy.DisplayName)
				if displayName == "" {
					return errors.Errorf("敌人组 enemy displayName 不能为空: group:%d enemy:%d %v",
						*group.ID, enemyID, xruntime.Location())
				}
				enemy.DisplayName = &displayName
			}
			if enemy.BattleAIID == nil || *enemy.BattleAIID == 0 {
				return errors.Errorf("敌人组 enemy 缺少有效 battleAI 引用: group:%d enemy:%d %v",
					*group.ID, enemyID, xruntime.Location())
			}
			if enemy.Level != nil && enemy.LevelRange != nil {
				return errors.Errorf("敌人组 enemy 不能同时配置 level 和 levelRange: group:%d enemy:%d %v",
					*group.ID, enemyID, xruntime.Location())
			}
			if enemy.Level != nil &&
				(*enemy.Level < uint32(pb.Constants_Constants_Level_Min) ||
					uint32(pb.Constants_Constants_Level_Max) < *enemy.Level) {
				return errors.Errorf("敌人组 enemy level 超出范围: group:%d enemy:%d level:%d %v",
					*group.ID, enemyID, *enemy.Level, xruntime.Location())
			}
			if enemy.LevelRange != nil &&
				(enemy.LevelRange.Min == nil || enemy.LevelRange.Max == nil ||
					*enemy.LevelRange.Min < int(pb.Constants_Constants_Level_Min) ||
					*enemy.LevelRange.Max > int(pb.Constants_Constants_Level_Max) ||
					*enemy.LevelRange.Min > *enemy.LevelRange.Max) {
				return errors.Errorf("敌人组 enemy levelRange 无效: group:%d enemy:%d %v",
					*group.ID, enemyID, xruntime.Location())
			}
			if enemy.GradeRange != nil &&
				(enemy.GradeRange.Min == nil || enemy.GradeRange.Max == nil ||
					*enemy.GradeRange.Min < int(pb.PetGrade_PetGrade_Common) ||
					*enemy.GradeRange.Max >= int(pb.PetGrade_PetGrade_Max) ||
					*enemy.GradeRange.Min > *enemy.GradeRange.Max) {
				return errors.Errorf("敌人组 enemy gradeRange 无效: group:%d enemy:%d %v",
					*group.ID, enemyID, xruntime.Location())
			}
			if enemy.AttributeModifiers != nil {
				for _, modifier := range []struct {
					field string
					value int32
				}{
					{field: "poisonResist", value: enemy.AttributeModifiers.PoisonResist},
					{field: "paralysisResist", value: enemy.AttributeModifiers.ParalysisResist},
					{field: "sleepResist", value: enemy.AttributeModifiers.SleepResist},
					{field: "stoneResist", value: enemy.AttributeModifiers.StoneResist},
					{field: "drunkResist", value: enemy.AttributeModifiers.DrunkResist},
					{field: "confusionResist", value: enemy.AttributeModifiers.ConfusionResist},
					{field: "critical", value: enemy.AttributeModifiers.Critical},
					{field: "counter", value: enemy.AttributeModifiers.Counter},
				} {
					if modifier.value < -100 || modifier.value > 100 {
						return errors.Errorf("敌人组 enemy attributeModifiers.%s 超出范围: group:%d enemy:%d value:%d expected:[-100,100] %v",
							modifier.field, *group.ID, enemyID, modifier.value, xruntime.Location())
					}
				}
			}
			if len(enemy.NormalDrops) > enemyNormalDropMaxCount {
				return errors.Errorf("敌人组 enemy normalDrops 超过最大槽位数量: group:%d enemy:%d size:%d %v",
					*group.ID, enemyID, len(enemy.NormalDrops), xruntime.Location())
			}
			for dropIndex := range enemy.NormalDrops {
				drop := &enemy.NormalDrops[dropIndex]
				if drop.ItemID == nil || (!isItemID(*drop.ItemID) && !isEquipmentID(*drop.ItemID)) {
					return errors.Errorf("敌人组 enemy normalDrops 道具ID无效: group:%d enemy:%d index:%d %v",
						*group.ID, enemyID, dropIndex, xruntime.Location())
				}
				if drop.Probability == nil || *drop.Probability == 0 || *drop.Probability > enemyNormalDropProbabilityMax {
					return errors.Errorf("敌人组 enemy normalDrops 万分比无效: group:%d enemy:%d item:%d index:%d %v",
						*group.ID, enemyID, *drop.ItemID, dropIndex, xruntime.Location())
				}
			}
			if *group.IsBoss {
				if enemy.Weight != nil {
					return errors.Errorf("Boss 敌人组不允许配置 weight: group:%d enemy:%d %v", *group.ID, enemyID, xruntime.Location())
				}
				if enemy.Level == nil && enemy.LevelRange == nil {
					return errors.Errorf("Boss 敌人组必须配置 level 或 levelRange: group:%d enemy:%d %v", *group.ID, enemyID, xruntime.Location())
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
			assetID := enemy.AssetID()
			if enemy.IsPet() && GGameConfig.Pet.Get(assetID) == nil {
				checkErr = errors.Errorf("敌人组引用了未定义宠物: group:%d pet:%d %v", *group.ID, assetID, xruntime.Location())
				return false
			}
			if enemy.IsCharacter() && GGameConfig.Character.Get(assetID) == nil {
				checkErr = errors.Errorf("敌人组引用了未定义角色: group:%d character:%d %v", *group.ID, assetID, xruntime.Location())
				return false
			}
			if GGameConfig.GrowthAttribute.Get(*enemy.GrowthAttributeID) == nil {
				checkErr = errors.Errorf("敌人组引用了未定义成长属性: group:%d enemy:%d growthAttribute:%d %v",
					*group.ID, assetID, *enemy.GrowthAttributeID, xruntime.Location())
				return false
			}
			if GGameConfig.AI == nil || GGameConfig.AI.Get(*enemy.BattleAIID) == nil {
				checkErr = errors.Errorf("敌人组引用了未定义AI: group:%d enemy:%d ai:%d %v",
					*group.ID, assetID, *enemy.BattleAIID, xruntime.Location())
				return false
			}
			for _, drop := range enemy.NormalDrops {
				if GGameConfig.Item == nil || GGameConfig.Item.Get(*drop.ItemID) == nil {
					checkErr = errors.Errorf("敌人组普通掉落引用了未定义道具: group:%d enemy:%d item:%d %v",
						*group.ID, assetID, *drop.ItemID, xruntime.Location())
					return false
				}
			}
		}
		return true
	})
	return checkErr
}

func (p *EnemyGroupConfig) assemble() error {
	p.Foreach(func(_ uint32, group *EnemyGroupEntry) bool {
		for index := range group.Enemies {
			enemy := &group.Enemies[index]
			enemy.BattleAI = GGameConfig.AI.Get(*enemy.BattleAIID)
			enemy.GrowthAttribute = GGameConfig.GrowthAttribute.Get(*enemy.GrowthAttributeID)
		}
		return true
	})
	return nil
}
