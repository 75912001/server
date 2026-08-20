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
	// SkillSlots 来自 pet.<family>[].skill, 按配置顺序保存技能槽位; 0 表示空槽, 非0值必须引用 skill.yaml 中存在的技能ID.
	SkillSlots []uint32 `yaml:"skill"`
	// BattleAI来自pet.<family>[].battleAI, 是玩家宠物模板和自动遇敌单位共用的AI配置.
	BattleAI *PetBattleAIEntry `yaml:"battleAI"`
}

type PetBattleAITargetScope string

const (
	PetBattleAITargetScopeAllOpponents     PetBattleAITargetScope = "allOpponents"
	PetBattleAITargetScopePlayerCharacters PetBattleAITargetScope = "playerCharacters"
	PetBattleAITargetScopePlayerPets       PetBattleAITargetScope = "playerPets"
	PetBattleAITargetScopePartyLeader      PetBattleAITargetScope = "partyLeader"
)

type PetBattleAITargetSelection string

const (
	PetBattleAITargetSelectionRandom          PetBattleAITargetSelection = "random"
	PetBattleAITargetSelectionHighestHP       PetBattleAITargetSelection = "highestHp"
	PetBattleAITargetSelectionLowestHP        PetBattleAITargetSelection = "lowestHp"
	PetBattleAITargetSelectionHighestAttack   PetBattleAITargetSelection = "highestAttack"
	PetBattleAITargetSelectionHighestAgility  PetBattleAITargetSelection = "highestAgility"
	PetBattleAITargetSelectionLowestAgility   PetBattleAITargetSelection = "lowestAgility"
	PetBattleAITargetSelectionElementalSubdue PetBattleAITargetSelection = "elementalSubdue"
)

type PetBattleAIEntry struct {
	// AttackWeight 是普通攻击相对权重, 范围[0,2147483647].
	AttackWeight *uint32 `yaml:"attackWeight"`
	// DefenseWeight 是本回合防御相对权重, 范围[0,2147483647].
	DefenseWeight *uint32 `yaml:"defenseWeight"`
	// EscapeWeight 是PVE逃跑相对权重, 范围[0,2147483647]. 逃跑是基础动作, 不是宠物技能.
	EscapeWeight *uint32 `yaml:"escapeWeight"`
	// SkillSlotWeights对应8.5 BATTLE_ai_normal的wa[0]至wa[6], 数组下标直接对应
	// PetEntry.SkillSlots的七个技能槽. 字段省略时等价于七项全0; 一旦配置就必须
	// 完整提供七项, 非0权重不能引用不存在或值为0的空技能槽.
	SkillSlotWeights []uint32 `yaml:"skillSlotWeights"`
	// TargetScope 限制普通攻击的候选目标类型.
	TargetScope *PetBattleAITargetScope `yaml:"targetScope"`
	// TargetSelection 定义在候选目标中选择最终目标的方式.
	TargetSelection *PetBattleAITargetSelection `yaml:"targetSelection"`
	// TargetRandomRollMax对应8.5 battle_ai.c的rn[0], 只允许非random目标策略配置.
	// 服务端先执行RAND(0, value): 结果为0时再随机选择候选, 其余结果使用策略最优目标.
	// 因此值1表示50%随机、50%最优, 值0表示每次都随机且仍保留两次RAND调用.
	TargetRandomRollMax *uint32 `yaml:"targetRandomRollMax"`
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
	// BaseVital 来自 growth.baseVital, 表示宠物模板固定基础体力值, 加品阶最小偏移后必须仍大于0.
	BaseVital *uint32 `yaml:"baseVital"`
	// BaseStr 来自 growth.baseStr, 表示宠物模板固定基础腕力/攻击值, 加品阶最小偏移后必须仍大于0.
	BaseStr *uint32 `yaml:"baseStr"`
	// BaseTough 来自 growth.baseTough, 表示宠物模板固定基础耐力/防御值, 加品阶最小偏移后必须仍大于0.
	BaseTough *uint32 `yaml:"baseTough"`
	// BaseDex 来自 growth.baseDex, 表示宠物模板固定基础速度/敏捷值, 加品阶最小偏移后必须仍大于0.
	BaseDex *uint32 `yaml:"baseDex"`
	// Rank 是加载 pet.yaml 后根据配置基础四维总和生成的成长档位, 不是 YAML 输入字段; 后续升级直接使用该值.
	Rank uint32 `yaml:"-"`
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
		if int32(*pet.Growth.BaseVital)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseVital 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v",
				*pet.ID, *pet.Growth.BaseVital, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(*pet.Growth.BaseStr)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseStr 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v",
				*pet.ID, *pet.Growth.BaseStr, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(*pet.Growth.BaseTough)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseTough 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v",
				*pet.ID, *pet.Growth.BaseTough, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		if int32(*pet.Growth.BaseDex)+petSavedBaseGradeOffsetMin <= 0 {
			return errors.Errorf("宠物 growth.baseDex 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d %v",
				*pet.ID, *pet.Growth.BaseDex, petSavedBaseGradeOffsetMin, xruntime.Location())
		}
		pet.Growth.Rank = petRankFromBaseSum(uint64(*pet.Growth.BaseVital) + uint64(*pet.Growth.BaseStr) + uint64(*pet.Growth.BaseTough) + uint64(*pet.Growth.BaseDex))

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

		if err := pet.BattleAI.check(*pet.ID, pet.SkillSlots); err != nil {
			return err
		}

		if !p.AddIfNotExist(*pet.ID, pet) {
			return errors.Errorf("宠物ID重复: %d %v", *pet.ID, xruntime.Location())
		}
	}
	return nil
}

// check校验单个敌方PVE基础AI配置.
//
// battleAI属于PetEntry自己的独立功能节点, 因此所有必填字段、数值边界和字段间
// 约束都在这里返回error. configure只负责补充宠物ID上下文并调用本方法, 避免把
// 每个功能的参数规则重新散落到宠物主加载循环中.
func (p *PetBattleAIEntry) check(petID uint32, skillSlots []uint32) error {
	if p == nil {
		return errors.Errorf("宠物缺少 battleAI: pet:%d %v", petID, xruntime.Location())
	}
	if p.AttackWeight == nil {
		return errors.Errorf("宠物缺少 battleAI.attackWeight: pet:%d %v", petID, xruntime.Location())
	}
	if p.DefenseWeight == nil {
		return errors.Errorf("宠物缺少 battleAI.defenseWeight: pet:%d %v", petID, xruntime.Location())
	}
	if p.EscapeWeight == nil {
		return errors.Errorf("宠物缺少 battleAI.escapeWeight: pet:%d %v", petID, xruntime.Location())
	}
	weightSum := uint64(*p.AttackWeight) + uint64(*p.DefenseWeight) + uint64(*p.EscapeWeight)
	if *p.AttackWeight > math.MaxInt32 || *p.DefenseWeight > math.MaxInt32 || *p.EscapeWeight > math.MaxInt32 {
		return errors.Errorf("宠物 battleAI 权重单项和总和必须在[0,2147483647]: pet:%d attack:%d defense:%d escape:%d %v",
			petID, *p.AttackWeight, *p.DefenseWeight, *p.EscapeWeight, xruntime.Location())
	}
	if p.SkillSlotWeights != nil {
		if len(p.SkillSlotWeights) != int(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
			return errors.Errorf("宠物 battleAI.skillSlotWeights 必须完整配置%d个槽位: pet:%d count:%d %v",
				pb.PetSkillLimit_PetSkillLimit_MaxSlotCount, petID, len(p.SkillSlotWeights), xruntime.Location())
		}
		for slotIndex, weight := range p.SkillSlotWeights {
			if weight > math.MaxInt32 {
				return errors.Errorf("宠物 battleAI.skillSlotWeights 权重超出[0,2147483647]: pet:%d slot:%d weight:%d %v",
					petID, slotIndex, weight, xruntime.Location())
			}
			if weight > 0 && (slotIndex >= len(skillSlots) || skillSlots[slotIndex] == 0) {
				return errors.Errorf("宠物 battleAI.skillSlotWeights 非0权重引用空技能槽: pet:%d slot:%d weight:%d %v",
					petID, slotIndex, weight, xruntime.Location())
			}
			weightSum += uint64(weight)
		}
	}
	if weightSum > math.MaxInt32 {
		return errors.Errorf("宠物 battleAI 动作权重总和超出[0,2147483647]: pet:%d total:%d %v",
			petID, weightSum, xruntime.Location())
	}
	if p.TargetScope == nil || (*p.TargetScope != PetBattleAITargetScopeAllOpponents &&
		*p.TargetScope != PetBattleAITargetScopePlayerCharacters &&
		*p.TargetScope != PetBattleAITargetScopePlayerPets &&
		*p.TargetScope != PetBattleAITargetScopePartyLeader) {
		return errors.Errorf("宠物 battleAI.targetScope 非法: pet:%d value:%v %v",
			petID, p.TargetScope, xruntime.Location())
	}
	if p.TargetSelection == nil || (*p.TargetSelection != PetBattleAITargetSelectionRandom &&
		*p.TargetSelection != PetBattleAITargetSelectionHighestHP &&
		*p.TargetSelection != PetBattleAITargetSelectionLowestHP &&
		*p.TargetSelection != PetBattleAITargetSelectionHighestAttack &&
		*p.TargetSelection != PetBattleAITargetSelectionHighestAgility &&
		*p.TargetSelection != PetBattleAITargetSelectionLowestAgility &&
		*p.TargetSelection != PetBattleAITargetSelectionElementalSubdue) {
		return errors.Errorf("宠物 battleAI.targetSelection 非法: pet:%d value:%v %v",
			petID, p.TargetSelection, xruntime.Location())
	}
	if *p.TargetSelection == PetBattleAITargetSelectionRandom {
		if p.TargetRandomRollMax != nil {
			return errors.Errorf("宠物 random目标策略不能配置 battleAI.targetRandomRollMax: pet:%d value:%d %v",
				petID, *p.TargetRandomRollMax, xruntime.Location())
		}
		return nil
	}
	if p.TargetRandomRollMax == nil {
		return errors.Errorf("宠物非random目标策略缺少 battleAI.targetRandomRollMax: pet:%d selection:%s %v",
			petID, *p.TargetSelection, xruntime.Location())
	}
	if *p.TargetRandomRollMax > math.MaxInt32 {
		return errors.Errorf("宠物 battleAI.targetRandomRollMax 超出[0,2147483647]: pet:%d value:%d %v",
			petID, *p.TargetRandomRollMax, xruntime.Location())
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
