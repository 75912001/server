package gameconfig

import (
	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"

	pb "server/proto/pb"
)

type PetConfig struct {
	*xmap.MapMgr[uint32, *PetEntry]
}

type PetEntry struct {
	// ID 来自 pet[].id, 必须处于协议宠物资源ID段内, 并且会被 enemy.group.yaml 的 enemies[].id 引用.
	ID *uint32 `yaml:"id"`
	// Rarity 来自 pet[].rarity, 使用协议 PetRarity 的整数值, 当前范围为普通到神话.
	Rarity *uint32 `yaml:"rarity"`
	// Elemental 来自 pet[].elemental, key 转为协议元素类型, 值范围[0,10], 总和必须为10.
	Elemental PetElementalEntry `yaml:"elemental"`
	// Attribute 来自 pet[].attribute, 保存宠物抗性和战斗附加属性, 字段必须为非负值.
	Attribute *PetAttributeEntry `yaml:"attribute"`
	// Growth 来自 pet[].growth, 保存宠物生成和升级时使用的基础成长参数.
	Growth *PetGrowthEntry `yaml:"growth"`
	// SkillSlots 来自 pet[].skill, 按配置顺序保存技能槽位; 0 表示空槽, 非0值必须引用 pet.skill.yaml 中存在的技能ID.
	SkillSlots []uint32 `yaml:"skill"`
}

type PetElementalEntry map[pb.AssetElemental]*uint32

type PetAttributeEntry struct {
	// PoisonResist 来自 attribute.poisonResist, 表示毒抗性修正值.
	PoisonResist *uint32 `yaml:"poisonResist"`
	// ParalysisResist 来自 attribute.paralysisResist, 表示麻痹抗性修正值.
	ParalysisResist *uint32 `yaml:"paralysisResist"`
	// SleepResist 来自 attribute.sleepResist, 表示睡眠抗性修正值.
	SleepResist *uint32 `yaml:"sleepResist"`
	// StoneResist 来自 attribute.stoneResist, 表示石化抗性修正值.
	StoneResist *uint32 `yaml:"stoneResist"`
	// DrunkResist 来自 attribute.drunkResist, 表示酒醉抗性修正值.
	DrunkResist *uint32 `yaml:"drunkResist"`
	// ConfusionResist 来自 attribute.confusionResist, 表示混乱抗性修正值.
	ConfusionResist *uint32 `yaml:"confusionResist"`
	// Critical 来自 attribute.critical, 表示暴击相关修正值.
	Critical *uint32 `yaml:"critical"`
	// Counter 来自 attribute.counter, 表示反击相关修正值.
	Counter *uint32 `yaml:"counter"`
	// Get 来自 attribute.get, 基础捕获值 (原版本)
	Get *uint32 `yaml:"get"`
	// Rare 来自 attribute.rate, 稀有度字段 (原版本)
	Rare *uint32 `yaml:"rate"`
}

type PetGrowthEntry struct {
	// InitNum 来自 growth.initNum, 表示初始系数, 参与 1 级宠物初始四维计算.
	InitNum *uint32 `yaml:"initNum"`
	// LvupPointSource 来自 growth.lvupPointSource, 表示原始升级成长点字段, 必须大于0.
	LvupPointSource *float64 `yaml:"lvupPointSource"`
	// BaseVital 来自 growth.baseVital, 表示宠物模板固定基础体力值, 加品阶最小偏移后必须仍大于0.
	BaseVital *uint32 `yaml:"baseVital"`
	// BaseStr 来自 growth.baseStr, 表示宠物模板固定基础腕力/攻击值, 加品阶最小偏移后必须仍大于0.
	BaseStr *uint32 `yaml:"baseStr"`
	// BaseTough 来自 growth.baseTough, 表示宠物模板固定基础耐力/防御值, 加品阶最小偏移后必须仍大于0.
	BaseTough *uint32 `yaml:"baseTough"`
	// BaseDex 来自 growth.baseDex, 表示宠物模板固定基础速度/敏捷值, 加品阶最小偏移后必须仍大于0.
	BaseDex *uint32 `yaml:"baseDex"`
}

func newPetConfig() *PetConfig {
	return &PetConfig{
		MapMgr: xmap.NewMapMgr[uint32, *PetEntry](),
	}
}

func (p *PetConfig) load(dir string) error {
	var root struct {
		Pet []*PetEntry `yaml:"pet"`
	}
	if err := loadYAMLFile(dir, FilePet, &root); err != nil {
		return err
	}
	return p.configure(root.Pet)
}

func (p *PetConfig) configure(entries []*PetEntry) error {
	for _, pet := range entries {
		if pet.ID == nil {
			return errors.Errorf("宠物缺少 id %v", xruntime.Location())
		}
		if !isPetID(*pet.ID) {
			return errors.Errorf("宠物ID超出范围: %d %v", *pet.ID, xruntime.Location())
		}
		if pet.Rarity == nil {
			return errors.Errorf("宠物缺少 rarity: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if *pet.Rarity < uint32(pb.PetRarity_PetRarity_Common) || *pet.Rarity > uint32(pb.PetRarity_PetRarity_Mythic) {
			return errors.Errorf("宠物稀有度非法: ID:%d rarity:%d %v", *pet.ID, *pet.Rarity, xruntime.Location())
		}
		if pet.Elemental == nil {
			return errors.Errorf("宠物缺少 elemental: pet:%d %v", *pet.ID, xruntime.Location())
		}
		for elementalType := pb.AssetElemental_AssetElemental_Unknow + 1; elementalType < pb.AssetElemental_AssetElemental_Max; elementalType++ {
			if pet.Elemental[elementalType] == nil {
				pet.Elemental[elementalType] = valuePtr(uint32(0))
			}
		}
		sum := uint32(0)
		activeIndexes := []int{}
		for elementalType := pb.AssetElemental_AssetElemental_Unknow + 1; elementalType < pb.AssetElemental_AssetElemental_Max; elementalType++ {
			value := *pet.Elemental[elementalType]
			if value > uint32(pb.ElementalLimit_ElementalLimit_TotalPoint) {
				return errors.Errorf("宠物 elemental 值必须在[0,10]: ID:%d value:%d %v", *pet.ID, value, xruntime.Location())
			}
			sum += value
			if value > 0 {
				index := int(elementalType - pb.AssetElemental_AssetElemental_Unknow - 1)
				activeIndexes = append(activeIndexes, index)
			}
		}
		if sum != uint32(pb.ElementalLimit_ElementalLimit_TotalPoint) {
			return errors.Errorf("宠物元素分配总和须为10: ID:%d sum:%d %v", *pet.ID, sum, xruntime.Location())
		}
		if len(activeIndexes) != 1 && len(activeIndexes) != 2 {
			return errors.Errorf("宠物 elemental 只能是单元素或两个相邻元素: ID:%d %v", *pet.ID, xruntime.Location())
		}
		if len(activeIndexes) == 2 {
			distance := activeIndexes[0] - activeIndexes[1]
			if distance < 0 {
				distance = -distance
			}
			wrapDistance := int(pb.AssetElemental_AssetElemental_Max - pb.AssetElemental_AssetElemental_Unknow - 2)
			if distance != 1 && distance != wrapDistance {
				return errors.Errorf("宠物 elemental 两个元素必须相邻: ID:%d %v", *pet.ID, xruntime.Location())
			}
		}

		if pet.Attribute == nil {
			return errors.Errorf("宠物缺少 attribute: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.PoisonResist == nil {
			return errors.Errorf("宠物缺少 attribute.poisonResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.ParalysisResist == nil {
			return errors.Errorf("宠物缺少 attribute.paralysisResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.SleepResist == nil {
			return errors.Errorf("宠物缺少 attribute.sleepResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.StoneResist == nil {
			return errors.Errorf("宠物缺少 attribute.stoneResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.DrunkResist == nil {
			return errors.Errorf("宠物缺少 attribute.drunkResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.ConfusionResist == nil {
			return errors.Errorf("宠物缺少 attribute.confusionResist: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.Critical == nil {
			return errors.Errorf("宠物缺少 attribute.critical: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.Counter == nil {
			return errors.Errorf("宠物缺少 attribute.counter: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.Get == nil {
			return errors.Errorf("宠物缺少 attribute.get: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Attribute.Rare == nil {
			return errors.Errorf("宠物缺少 attribute.rate: pet:%d %v", *pet.ID, xruntime.Location())
		}

		if pet.Growth == nil {
			return errors.Errorf("宠物缺少 growth: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Growth.InitNum == nil {
			return errors.Errorf("宠物缺少 growth.initNum: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Growth.LvupPointSource == nil {
			return errors.Errorf("宠物缺少 growth.lvupPointSource: pet:%d %v", *pet.ID, xruntime.Location())
		}
		lvupPointSource := *pet.Growth.LvupPointSource
		if lvupPointSource <= 0 {
			return errors.Errorf("宠物 growth.lvupPointSource 必须大于0: ID:%d value:%v %v", *pet.ID, lvupPointSource, xruntime.Location())
		}
		if pet.Growth.BaseVital == nil {
			return errors.Errorf("宠物缺少 growth.baseVital: pet:%d %v", *pet.ID, xruntime.Location())
		}
		baseVital := *pet.Growth.BaseVital
		if pet.Growth.BaseStr == nil {
			return errors.Errorf("宠物缺少 growth.baseStr: pet:%d %v", *pet.ID, xruntime.Location())
		}
		baseStr := *pet.Growth.BaseStr
		if pet.Growth.BaseTough == nil {
			return errors.Errorf("宠物缺少 growth.baseTough: pet:%d %v", *pet.ID, xruntime.Location())
		}
		baseTough := *pet.Growth.BaseTough
		if pet.Growth.BaseDex == nil {
			return errors.Errorf("宠物缺少 growth.baseDex: pet:%d %v", *pet.ID, xruntime.Location())
		}
		baseDex := *pet.Growth.BaseDex
		if int32(baseVital)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseVital 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v", *pet.ID, baseVital, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(baseStr)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseStr 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v", *pet.ID, baseStr, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(baseTough)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseTough 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v", *pet.ID, baseTough, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(baseDex)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseDex 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v", *pet.ID, baseDex, petSavedBaseGradeOffsetMin, xruntime.Location())
		}

		if pet.SkillSlots == nil {
			return errors.Errorf("宠物缺少 skill: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if len(pet.SkillSlots) == 0 {
			return errors.Errorf("宠物 skill 须大于 0 个槽位: ID:%d %v", *pet.ID, xruntime.Location())
		}
		hasSkill := false
		for index, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			hasSkill = true
			if !isPetSkillID(skillID) {
				return errors.Errorf("宠物 skill 槽位ID超出范围: ID:%d skill:%d index:%d %v", *pet.ID, skillID, index, xruntime.Location())
			}
		}
		if !hasSkill {
			return errors.Errorf("宠物 skill 至少需要一个非0技能: ID:%d %v", *pet.ID, xruntime.Location())
		}

		if !p.AddIfNotExist(*pet.ID, pet) {
			return errors.Errorf("宠物ID重复: %d %v", *pet.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *PetConfig) check() error {
	var err error
	p.Foreach(func(petID uint32, pet *PetEntry) bool {
		for _, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			if !GGameConfig.PetSkill.IsExist(skillID) {
				err = errors.Errorf("宠物引用了未定义技能: pet:%d skill:%d %v", petID, skillID, xruntime.Location())
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *PetConfig) assemble() error {
	return nil
}
