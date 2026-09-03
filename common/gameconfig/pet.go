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

type PetConfig struct {
	*xmap.MapMgr[uint32, *PetEntry]
}

type PetEntry struct {
	// ID 来自 pet.<family>[].id, 必须处于协议宠物资源ID段内, 并且可被enemy.group.yaml直接引用.
	ID *uint32 `yaml:"id"`
	// Name 来自 pet.<family>[].name, 仅用于配置核验错误日志定位具体宠物.
	Name *string `yaml:"name"`
	// CreationMode 区分普通宠物模板和融合蛋模板. 省略表示普通创建; fusionEgg 只保留原版模板数据, 必须走后续独立融合流程.
	CreationMode PetCreationMode `yaml:"creationMode,omitempty"`
	// Rarity 来自 pet.<family>[].rarity, 使用协议 PetRarity 的整数值, 当前范围为普通到神话.
	Rarity *uint32 `yaml:"rarity"`
	// Elemental 来自 pet.<family>[].elemental, key 转为协议元素类型, 值范围[0,10], 总和必须为10.
	Elemental PetElementalEntry `yaml:"elemental"`
	// Attribute 来自 pet.<family>[].attribute, 保存宠物抗性和战斗附加属性, 字段必须为非负值.
	Attribute *PetAttributeEntry `yaml:"attribute"`
	// Growth 来自 pet.<family>[].growth, 保存宠物生成和升级时使用的基础成长参数.
	Growth *PetGrowthEntry `yaml:"growth"`
	// PanelReference 保存供客户端图鉴直接展示的预计算四维和总成长参考值; 服务端启动时按权威成长规则重新计算并逐项核验.
	PanelReference *PetPanelReferenceEntry `yaml:"panelReference"`
	// SkillSlots 保存新宠物出生时的固定七槽技能; 0表示空槽, 创建后以实例技能为准.
	SkillSlots []uint32 `yaml:"skill"`
}

// UnmarshalYAML拒绝旧的宠物AI字段, 防止迁移遗漏被YAML解析器静默忽略.
func (p *PetEntry) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == "battleAI" {
			return errors.New("pet.yaml 不再允许 battleAI, 请在 enemy.group.yaml 的敌人条目中配置")
		}
	}
	type petEntry PetEntry
	return node.Decode((*petEntry)(p))
}

type PetCreationMode string

const (
	PetCreationModeOrdinary  PetCreationMode = ""
	PetCreationModeFusionEgg PetCreationMode = "fusionEgg"
)

// SupportsOrdinaryCreation 返回该模板能否进入普通宠物生成链路.
func (p *PetEntry) SupportsOrdinaryCreation() bool {
	return p != nil && p.CreationMode == PetCreationModeOrdinary
}

type PetElementalEntry map[pb.AssetElemental]*uint32

var petElementalByYAMLKey = map[string]pb.AssetElemental{
	"earth": pb.AssetElemental_AssetElemental_Earth,
	"water": pb.AssetElemental_AssetElemental_Water,
	"fire":  pb.AssetElemental_AssetElemental_Fire,
	"wind":  pb.AssetElemental_AssetElemental_Wind,
}

// UnmarshalYAML 将共享配置使用的元素名称转换为服务端协议枚举, 未知名称直接返回错误.
func (p *PetElementalEntry) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]*uint32
	if err := node.Decode(&raw); err != nil {
		return err
	}

	parsed := make(PetElementalEntry, len(raw))
	for key, value := range raw {
		elemental, ok := petElementalByYAMLKey[key]
		if !ok {
			return errors.Errorf("宠物 elemental 元素未知: %s", key)
		}
		parsed[elemental] = value
	}
	*p = parsed
	return nil
}

type PetAttributeEntry struct {
	// PoisonResist 来自 attribute.poisonResist, 表示毒抗性修正值.
	PoisonResist *int32 `yaml:"poisonResist"`
	// ParalysisResist 来自 attribute.paralysisResist, 表示麻痹抗性修正值.
	ParalysisResist *int32 `yaml:"paralysisResist"`
	// SleepResist 来自 attribute.sleepResist, 表示睡眠抗性修正值.
	SleepResist *int32 `yaml:"sleepResist"`
	// StoneResist 来自 attribute.stoneResist, 表示石化抗性修正值.
	StoneResist *int32 `yaml:"stoneResist"`
	// DrunkResist 来自 attribute.drunkResist, 表示酒醉抗性修正值.
	DrunkResist *int32 `yaml:"drunkResist"`
	// ConfusionResist 来自 attribute.confusionResist, 表示混乱抗性修正值.
	ConfusionResist *int32 `yaml:"confusionResist"`
	// Critical 来自 attribute.critical, 表示暴击相关修正值.
	Critical *uint32 `yaml:"critical"`
	// Counter 来自 attribute.counter, 表示反击相关修正值.
	Counter *uint32 `yaml:"counter"`
	// Get 来自 attribute.get, 基础捕获值 (原版本)
	Get *int32 `yaml:"get"`
	// Rare 来自 attribute.rate, 稀有度字段 (原版本)
	Rare *uint32 `yaml:"rate"`
	// UltimateKnockbackImmune 映射8.5中真实基础形象101813/101814的雷尔特性.
	// 该值只由服务端配置决定: true时仍执行单次/累计过量伤害的历史记账顺序,
	// 但最终不产生Ultimate击飞, 也不会因为被排除的击飞而清空历史累计值.
	UltimateKnockbackImmune bool `yaml:"ultimateKnockbackImmune"`
	// Inanimate 映射8.5 CHAR_BATTLEFLG_ABIO“无生物”标记. 旧服只给真实
	// 基础形象100466至100471设置该标记; 其单位死亡时强制使用Ultimate类型1,
	// 并跳过非玩家暴击死亡的50%随机判定. 其他ABIO战斗规则由各自批次读取.
	Inanimate bool `yaml:"inanimate"`
}

type PetGrowthEntry struct {
	// InitNum 来自 growth.initNum, 表示初始系数, 参与 1 级宠物初始四维计算.
	InitNum *uint32 `yaml:"initNum"`
	// LvupPointSource 来自 growth.lvupPointSource, 表示原始升级成长点字段, 必须大于0.
	LvupPointSource *float64 `yaml:"lvupPointSource"`
	// BaseVital 来自 growth.baseVital, 表示宠物模板固定基础体力值; 品阶和随机点使用有符号中间值计算.
	BaseVital *uint32 `yaml:"baseVital"`
	// BaseStr 来自 growth.baseStr, 表示宠物模板固定基础腕力/攻击值; 品阶和随机点使用有符号中间值计算.
	BaseStr *uint32 `yaml:"baseStr"`
	// BaseTough 来自 growth.baseTough, 表示宠物模板固定基础耐力/防御值; 品阶和随机点使用有符号中间值计算.
	BaseTough *uint32 `yaml:"baseTough"`
	// BaseDex 来自 growth.baseDex, 表示宠物模板固定基础速度/敏捷值; 品阶和随机点使用有符号中间值计算.
	BaseDex *uint32 `yaml:"baseDex"`
	// Rank 是加载 pet.yaml 后根据配置基础四维总和生成的成长档位, 不是 YAML 输入字段; 后续升级直接使用该值.
	Rank uint32 `yaml:"-"`
}

// validatePetSignedRawRange验证普通创建和逐级升级的全部随机结果都能写入PetRecord的int32 Raw字段.
// 基础值本身允许在叠加品阶偏移后为0或负数; 这里只拒绝真正越过协议整数边界的配置.
func validatePetSignedRawRange(pet *PetEntry) error {
	initNum := int64(*pet.Growth.InitNum)
	if initNum <= 0 || initNum > math.MaxInt32 {
		return errors.Errorf("宠物 growth.initNum 超出有符号计算范围: ID:%d value:%d %v",
			*pet.ID, *pet.Growth.InitNum, xruntime.Location())
	}

	rankMin, rankMax := PetRankGrowthRange(pet.Growth.Rank)
	upgradeCount := int64(pb.LevelRange_LevelRange_Max - pb.LevelRange_LevelRange_Min)
	attributes := []struct {
		name  string
		value uint32
	}{
		{name: "baseVital", value: *pet.Growth.BaseVital},
		{name: "baseStr", value: *pet.Growth.BaseStr},
		{name: "baseTough", value: *pet.Growth.BaseTough},
		{name: "baseDex", value: *pet.Growth.BaseDex},
	}
	for _, attribute := range attributes {
		minimumSavedBase := int64(attribute.value) + int64(petSavedBaseGradeOffsetMin)
		maximumSavedBase := int64(attribute.value) + int64(petSavedBaseGradeOffsetMax)
		if minimumSavedBase < math.MinInt32 || maximumSavedBase > math.MaxInt32 {
			return errors.Errorf("宠物 growth.%s 品阶偏移后超出int32: ID:%d value:%d %v",
				attribute.name, *pet.ID, attribute.value, xruntime.Location())
		}

		minimumUpgradeMultiplier := rankMin
		if minimumSavedBase < 0 {
			minimumUpgradeMultiplier = rankMax
		}
		minimumUpgrade := int64(float64(minimumSavedBase) * minimumUpgradeMultiplier)
		maximumRandomBase := maximumSavedBase + 10
		maximumUpgrade := int64(float64(maximumRandomBase) * rankMax)
		minimumRaw := minimumSavedBase*initNum + minimumUpgrade*upgradeCount
		maximumRaw := maximumRandomBase*initNum + maximumUpgrade*upgradeCount
		if minimumRaw < math.MinInt32 || maximumRaw > math.MaxInt32 {
			return errors.Errorf("宠物 growth.%s 在1至%d级随机范围内超出int32: ID:%d min:%d max:%d %v",
				attribute.name, pb.LevelRange_LevelRange_Max, *pet.ID, minimumRaw, maximumRaw, xruntime.Location())
		}
	}
	return nil
}

func newPetConfig() *PetConfig {
	return &PetConfig{
		MapMgr: xmap.NewMapMgr[uint32, *PetEntry](),
	}
}

func (p *PetConfig) load(dir string) error {
	// root 只适配 YAML 的系别结构; 校验后逐组写入 ID 索引, 运行时不保留系别.
	var root struct {
		Pet map[string][]*PetEntry `yaml:"pet"`
	}
	if err := loadYAMLFile(dir, FilePet, &root); err != nil {
		return err
	}
	if len(root.Pet) == 0 {
		return errors.Errorf("宠物配置 pet 段不能为空 %v", xruntime.Location())
	}
	for family, entries := range root.Pet {
		trimmedFamily := strings.TrimSpace(family)
		if trimmedFamily == "" {
			return errors.Errorf("宠物系别不能为空 %v", xruntime.Location())
		}
		if trimmedFamily != family {
			return errors.Errorf("宠物系别首尾不能包含空白: %q %v", family, xruntime.Location())
		}
		if len(entries) == 0 {
			return errors.Errorf("宠物系别没有配置宠物: family:%s %v", family, xruntime.Location())
		}
	}
	for _, entries := range root.Pet {
		if err := p.configure(entries); err != nil {
			return err
		}
	}
	panelReferenceErrors := make([]string, 0)
	for _, pets := range root.Pet {
		for _, pet := range pets {
			expected := calculatePetPanelReference(pet)
			if !petPanelReferenceEqual(pet.PanelReference, &expected) {
				panelReferenceErrors = append(panelReferenceErrors, formatPetPanelReferenceError(pet, &expected))
			}
		}
	}
	if len(panelReferenceErrors) > 0 {
		return errors.Errorf("宠物 panelReference 核验失败: count:%d\n%s %v",
			len(panelReferenceErrors), strings.Join(panelReferenceErrors, "\n"), xruntime.Location())
	}
	return nil
}

func (p *PetConfig) configure(entries []*PetEntry) error {
	for _, pet := range entries {
		if pet.ID == nil {
			return errors.Errorf("宠物缺少 id %v", xruntime.Location())
		}
		if !isPetID(*pet.ID) {
			return errors.Errorf("宠物ID超出范围: %d %v", *pet.ID, xruntime.Location())
		}
		if pet.Name == nil || strings.TrimSpace(*pet.Name) == "" {
			return errors.Errorf("宠物缺少 name: pet:%d %v", *pet.ID, xruntime.Location())
		}
		switch pet.CreationMode {
		case PetCreationModeOrdinary, PetCreationModeFusionEgg:
		default:
			return errors.Errorf("宠物 creationMode 非法: ID:%d mode:%q %v", *pet.ID, pet.CreationMode, xruntime.Location())
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
			if value > uint32(pb.Constants_Constants_Elemental_Total_Point) {
				return errors.Errorf("宠物 elemental 值必须在[0,10]: ID:%d value:%d %v", *pet.ID, value, xruntime.Location())
			}
			sum += value
			if value > 0 {
				index := int(elementalType - pb.AssetElemental_AssetElemental_Unknow - 1)
				activeIndexes = append(activeIndexes, index)
			}
		}
		if sum != uint32(pb.Constants_Constants_Elemental_Total_Point) {
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
		if pet.Growth.BaseStr == nil {
			return errors.Errorf("宠物缺少 growth.baseStr: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Growth.BaseTough == nil {
			return errors.Errorf("宠物缺少 growth.baseTough: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if pet.Growth.BaseDex == nil {
			return errors.Errorf("宠物缺少 growth.baseDex: pet:%d %v", *pet.ID, xruntime.Location())
		}
		pet.Growth.Rank = petRankFromBaseSum(uint64(*pet.Growth.BaseVital) + uint64(*pet.Growth.BaseStr) + uint64(*pet.Growth.BaseTough) + uint64(*pet.Growth.BaseDex))
		if err := validatePetSignedRawRange(pet); err != nil {
			return err
		}

		if pet.SkillSlots == nil {
			return errors.Errorf("宠物缺少 skill: pet:%d %v", *pet.ID, xruntime.Location())
		}
		if len(pet.SkillSlots) != int(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
			return errors.Errorf("宠物 skill 必须完整配置%d个槽位: ID:%d count:%d %v",
				pb.PetSkillLimit_PetSkillLimit_MaxSlotCount, *pet.ID, len(pet.SkillSlots), xruntime.Location())
		}
		hasSkill := false
		for index, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			hasSkill = true
			if !isSkillID(skillID) {
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

// petRankFromBaseSum 根据宠物配置表的固定基础四维总和生成成长档位.
func petRankFromBaseSum(baseSum uint64) uint32 {
	if baseSum >= 100 {
		return 0
	}
	if baseSum >= 95 {
		return 1
	}
	if baseSum >= 90 {
		return 2
	}
	if baseSum >= 85 {
		return 3
	}
	if baseSum >= 80 {
		return 4
	}
	return 5
}

func (p *PetConfig) check() error {
	var err error
	p.Foreach(func(petID uint32, pet *PetEntry) bool {
		for _, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			if !GGameConfig.Skill.IsExist(skillID) {
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
