package gameconfig

import (
	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type SkillConfig struct {
	*xmap.MapMgr[uint32, *SkillEntry]
}

type SkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议技能共用资源ID段内, 并且在 技能.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// UsableBy保存允许使用技能的玩家单位类型, 当前只接受character和pet.
	UsableBy []string `yaml:"usableBy"`
	// MPCost 和 TargetScope 由主动精灵技能自身持有, 装备只负责授予技能ID.
	MPCost      *uint32 `yaml:"mpCost"`
	TargetScope string  `yaml:"targetScope"`
	// ContinuationAttack 非 nil 表示连续攻击, 段数由服务端配置决定.
	ContinuationAttack *SkillContinuationAttackEntry `yaml:"continuationAttack"`
	// MightyAttack 非 nil 表示一击必杀, 只修正本次主动攻击的最终伤害和目标闪避.
	MightyAttack *SkillMightyAttackEntry `yaml:"mightyAttack"`
	// PoisonAttack 非 nil 表示物理攻击附加普通中毒, 毒伤由目标基础四维决定.
	PoisonAttack *SkillPoisonAttackEntry `yaml:"poisonAttack"`
	// StoneAttack 非 nil 表示物理攻击附加普通石化, 时长按原版写入运行态时另加1.
	StoneAttack *SkillStoneAttackEntry `yaml:"stoneAttack"`
	// ConfusionAttack 非 nil 表示物理攻击附加普通混乱, 时长按原版写入运行态时另加1.
	ConfusionAttack *SkillConfusionAttackEntry `yaml:"confusionAttack"`
	// SleepAttack 非 nil 表示物理攻击附加普通睡眠, 时长按原版写入运行态时另加1.
	SleepAttack *SkillSleepAttackEntry `yaml:"sleepAttack"`
	// DeepPoisonAttack 非 nil 表示物理攻击附加剧毒, 到期在目标行动前强制致死.
	DeepPoisonAttack *SkillDeepPoisonAttackEntry `yaml:"deepPoisonAttack"`
	// ChargeAttack 非 nil 表示先蓄力若干回合, 再以提高后的基础攻击力执行一次物理攻击.
	ChargeAttack *SkillChargeAttackEntry `yaml:"chargeAttack"`
	// EarthRound 非 nil 表示本回合隐藏, 下一回合从敌后现身并放大最终物理伤害.
	EarthRound *SkillEarthRoundEntry `yaml:"earthRound"`
	// Guardian 非 nil 表示本回合以修正后的攻防执行普通物理攻击, 并保护同列前排主人.
	Guardian *SkillGuardianEntry `yaml:"guardian"`
	// NoGuard 非 nil 表示本回合不主动攻击, 但提高自身闪避和反击判定.
	NoGuard *SkillNoGuardEntry `yaml:"noGuard"`
	// PowerBalance 非 nil 表示本回合以修正后的攻防执行一次普通物理攻击.
	PowerBalance *SkillPowerBalanceEntry `yaml:"powerBalance"`
	// ShowMercy 非 nil 表示普通单段物理攻击只扣到目标剩1HP, 不赋予持续保命状态.
	ShowMercy *SkillShowMercyEntry `yaml:"showMercy"`
	// Abduct 非 nil 表示旅程伙伴: 目标和施法宠按服务端结果离场, 不进入捕获或宠物档案流程.
	Abduct *SkillAbductEntry `yaml:"abduct"`
	// Processing 非 nil 表示地图上的宠物加工能力, YAML只接受空对象.
	Processing *SkillProcessingEntry `yaml:"processing"`
	// 五系异常精灵分别保留独立参数块, 便于在技能编辑器中聚合查看和修改单个技能.
	PoisonSpirit    *SkillPoisonSpiritEntry    `yaml:"poisonSpirit"`
	StoneSpirit     *SkillStoneSpiritEntry     `yaml:"stoneSpirit"`
	ConfusionSpirit *SkillConfusionSpiritEntry `yaml:"confusionSpirit"`
	DrunkSpirit     *SkillDrunkSpiritEntry     `yaml:"drunkSpirit"`
	SleepSpirit     *SkillSleepSpiritEntry     `yaml:"sleepSpirit"`
	// 三种治疗精灵分别保存完整参数, 避免单个技能的治疗量与表现配置分散到外部表.
	HealingSpirit  *SkillHealingSpiritEntry  `yaml:"healingSpirit"`
	MoistureSpirit *SkillMoistureSpiritEntry `yaml:"moistureSpirit"`
	GraceSpirit    *SkillGraceSpiritEntry    `yaml:"graceSpirit"`
}

type SkillContinuationAttackEntry struct {
	SegmentCount *uint32 `yaml:"segmentCount"`
}

func (p *SkillContinuationAttackEntry) check() error {
	if p.SegmentCount == nil {
		return errors.Errorf("连续攻击缺少 continuationAttack.segmentCount %v", xruntime.Location())
	}
	if *p.SegmentCount < 1 || *p.SegmentCount > 10 {
		return errors.Errorf("连续攻击 continuationAttack.segmentCount 超出1至10范围: value:%d %v", *p.SegmentCount, xruntime.Location())
	}
	return nil
}

type SkillMightyAttackEntry struct {
	DamageMultiplier *uint32 `yaml:"damageMultiplier"`
	TargetDodgeBonus *uint32 `yaml:"targetDodgeBonus"`
}

type SkillPoisonAttackEntry struct {
	DurationActions       *uint32 `yaml:"durationActions"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillStoneAttackEntry struct {
	DurationActions       *uint32 `yaml:"durationActions"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillConfusionAttackEntry struct {
	DurationActions       *uint32 `yaml:"durationActions"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillSleepAttackEntry struct {
	DurationActions       *uint32 `yaml:"durationActions"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillDeepPoisonAttackEntry struct {
	DurationActions       *uint32 `yaml:"durationActions"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillChargeAttackEntry struct {
	ChargeRounds          *uint32 `yaml:"chargeRounds"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

type SkillEarthRoundEntry struct {
	DamagePercentModifier *int32 `yaml:"damagePercentModifier"`
}

type SkillGuardianEntry struct {
	AttackPercentModifier  *int32 `yaml:"attackPercentModifier"`
	DefensePercentModifier *int32 `yaml:"defensePercentModifier"`
}

type SkillNoGuardEntry struct {
	DodgePercent    *int32  `yaml:"dodgePercent"`
	CounterPercent  *uint32 `yaml:"counterPercent"`
	CriticalPercent *uint32 `yaml:"criticalPercent"`
}

type SkillPowerBalanceEntry struct {
	AttackPercentModifier  *int32 `yaml:"attackPercentModifier"`
	DefensePercentModifier *int32 `yaml:"defensePercentModifier"`
}

// SkillShowMercyEntry是固定留1HP的无参数标记, YAML只接受空对象.
type SkillShowMercyEntry struct{}

// SkillAbductEntry保存旅程伙伴2/3对玩家战宠目标生效的可选忠诚度阈值.
// 空对象表示原版130的等级差公式; 阈值只在目标属于玩家战宠时替代该公式.
type SkillAbductEntry struct {
	LoyaltyThreshold *uint32 `yaml:"loyaltyThreshold"`
}

func (p *SkillAbductEntry) check() error {
	if p.LoyaltyThreshold != nil && (*p.LoyaltyThreshold < 1 || *p.LoyaltyThreshold > 100) {
		return errors.Errorf("旅程伙伴 abduct.loyaltyThreshold 超出1至100范围: value:%d %v", *p.LoyaltyThreshold, xruntime.Location())
	}
	return nil
}

// SkillProcessingEntry是无参数的宠物加工标记.
type SkillProcessingEntry struct{}

type SkillPoisonSpiritEntry struct {
	StatusID             *uint32 `yaml:"statusId"`
	DurationActions      *uint32 `yaml:"durationActions"`
	BaseSuccess          *uint32 `yaml:"baseSuccess"`
	LevelDifferenceRange *uint32 `yaml:"levelDifferenceRange"`
	CastEffectID         *uint32 `yaml:"castEffectId"`
	ReceiveEffectID      *uint32 `yaml:"receiveEffectId"`
	StatusEffectID       *uint32 `yaml:"statusEffectId"`
}

type SkillStoneSpiritEntry SkillPoisonSpiritEntry
type SkillConfusionSpiritEntry SkillPoisonSpiritEntry
type SkillDrunkSpiritEntry SkillPoisonSpiritEntry
type SkillSleepSpiritEntry SkillPoisonSpiritEntry

type SkillHealingSpiritEntry struct {
	HealPower    *uint32 `yaml:"healPower"`
	CastEffectID *uint32 `yaml:"castEffectId"`
	HealEffectID *uint32 `yaml:"healEffectId"`
}

type SkillMoistureSpiritEntry SkillHealingSpiritEntry
type SkillGraceSpiritEntry SkillHealingSpiritEntry

type SkillHealingSpiritParameters struct {
	HealPower    uint32
	CastEffectID uint32
	HealEffectID uint32
}

// HealingSpirit返回三种独立治疗配置块中唯一存在的一种.
func (p *SkillEntry) HealingSpiritParameters() (*SkillHealingSpiritParameters, bool) {
	if p == nil {
		return nil, false
	}
	var entry *SkillHealingSpiritEntry
	switch {
	case p.HealingSpirit != nil:
		entry = p.HealingSpirit
	case p.MoistureSpirit != nil:
		entry = (*SkillHealingSpiritEntry)(p.MoistureSpirit)
	case p.GraceSpirit != nil:
		entry = (*SkillHealingSpiritEntry)(p.GraceSpirit)
	default:
		return nil, false
	}
	if entry.HealPower == nil || entry.CastEffectID == nil || entry.HealEffectID == nil {
		return nil, false
	}
	return &SkillHealingSpiritParameters{
		HealPower:    *entry.HealPower,
		CastEffectID: *entry.CastEffectID,
		HealEffectID: *entry.HealEffectID,
	}, true
}

func checkHealingSpirit(name string, entry *SkillHealingSpiritEntry) error {
	if entry == nil || entry.HealPower == nil || entry.CastEffectID == nil || entry.HealEffectID == nil {
		return errors.Errorf("%s缺少完整治疗参数 %v", name, xruntime.Location())
	}
	if *entry.HealPower == 0 {
		return errors.Errorf("%s healPower必须大于0 %v", name, xruntime.Location())
	}
	if *entry.CastEffectID == 0 || *entry.HealEffectID == 0 {
		return errors.Errorf("%s表现资源ID不能为0 %v", name, xruntime.Location())
	}
	return nil
}

type SkillStatusSpiritParameters struct {
	StatusID             uint32
	DurationActions      uint32
	BaseSuccess          uint32
	LevelDifferenceRange uint32
	CastEffectID         uint32
	ReceiveEffectID      uint32
	StatusEffectID       uint32
}

// StatusSpirit 返回五种独立配置块中唯一存在的一种, 供战斗层统一搬运已校验参数.
func (p *SkillEntry) StatusSpirit() (*SkillStatusSpiritParameters, bool) {
	if p == nil {
		return nil, false
	}
	var entry *SkillPoisonSpiritEntry
	switch {
	case p.PoisonSpirit != nil:
		entry = p.PoisonSpirit
	case p.StoneSpirit != nil:
		entry = (*SkillPoisonSpiritEntry)(p.StoneSpirit)
	case p.ConfusionSpirit != nil:
		entry = (*SkillPoisonSpiritEntry)(p.ConfusionSpirit)
	case p.DrunkSpirit != nil:
		entry = (*SkillPoisonSpiritEntry)(p.DrunkSpirit)
	case p.SleepSpirit != nil:
		entry = (*SkillPoisonSpiritEntry)(p.SleepSpirit)
	default:
		return nil, false
	}
	if entry.StatusID == nil || entry.DurationActions == nil || entry.BaseSuccess == nil || entry.LevelDifferenceRange == nil ||
		entry.CastEffectID == nil || entry.ReceiveEffectID == nil || entry.StatusEffectID == nil {
		return nil, false
	}
	return &SkillStatusSpiritParameters{
		StatusID:             *entry.StatusID,
		DurationActions:      *entry.DurationActions,
		BaseSuccess:          *entry.BaseSuccess,
		LevelDifferenceRange: *entry.LevelDifferenceRange,
		CastEffectID:         *entry.CastEffectID,
		ReceiveEffectID:      *entry.ReceiveEffectID,
		StatusEffectID:       *entry.StatusEffectID,
	}, true
}

func checkStatusSpirit(name string, entry *SkillPoisonSpiritEntry, expectedStatusID uint32) error {
	if entry == nil || entry.StatusID == nil || entry.DurationActions == nil || entry.BaseSuccess == nil ||
		entry.LevelDifferenceRange == nil || entry.CastEffectID == nil || entry.ReceiveEffectID == nil || entry.StatusEffectID == nil {
		return errors.Errorf("%s缺少完整异常状态参数 %v", name, xruntime.Location())
	}
	if *entry.StatusID != expectedStatusID {
		return errors.Errorf("%s statusId必须为%d: value:%d %v", name, expectedStatusID, *entry.StatusID, xruntime.Location())
	}
	if *entry.DurationActions < 1 || *entry.DurationActions > 32767 {
		return errors.Errorf("%s durationActions超出1至32767范围: value:%d %v", name, *entry.DurationActions, xruntime.Location())
	}
	if *entry.BaseSuccess > 100 || *entry.LevelDifferenceRange > 100 {
		return errors.Errorf("%s baseSuccess或levelDifferenceRange超出0至100范围 %v", name, xruntime.Location())
	}
	if *entry.CastEffectID == 0 || *entry.ReceiveEffectID == 0 || *entry.StatusEffectID == 0 {
		return errors.Errorf("%s表现资源ID不能为0 %v", name, xruntime.Location())
	}
	return nil
}

// CanBeUsedBy判断技能运行配置是否允许指定玩家单位类型使用.
func (p *SkillEntry) CanBeUsedBy(unitType string) bool {
	for _, allowedType := range p.UsableBy {
		if allowedType == unitType {
			return true
		}
	}
	return false
}

// UnmarshalYAML拒绝技能参数对象的空值和非整数参数, 避免YAML解码静默截断小数.
func (p *SkillEntry) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		behavior := node.Content[index].Value
		if behavior == "cost" {
			return errors.New("技能 cost 已迁移到 商店.yaml, 技能.yaml 不再接受该字段")
		}
		statusSpirit := behavior == "poisonSpirit" || behavior == "stoneSpirit" || behavior == "confusionSpirit" || behavior == "drunkSpirit" || behavior == "sleepSpirit"
		healingSpirit := behavior == "healingSpirit" || behavior == "moistureSpirit" || behavior == "graceSpirit"
		if behavior != "mightyAttack" && behavior != "poisonAttack" && behavior != "stoneAttack" && behavior != "confusionAttack" && behavior != "sleepAttack" && behavior != "deepPoisonAttack" && behavior != "chargeAttack" && behavior != "earthRound" && behavior != "guardian" && behavior != "noGuard" && behavior != "powerBalance" && behavior != "showMercy" && behavior != "abduct" && behavior != "processing" && !statusSpirit && !healingSpirit {
			continue
		}
		parameters := node.Content[index+1]
		if parameters.Kind == yaml.AliasNode {
			parameters = parameters.Alias
		}
		if parameters.Kind != yaml.MappingNode {
			return errors.Errorf("技能 %s 必须为对象", behavior)
		}
		if (behavior == "showMercy" || behavior == "processing") && len(parameters.Content) != 0 {
			return errors.Errorf("技能 %s 必须为空对象, 不接受参数", behavior)
		}
		for field := 0; field+1 < len(parameters.Content); field += 2 {
			name := parameters.Content[field].Value
			if behavior == "abduct" && name != "loyaltyThreshold" {
				return errors.Errorf("技能 abduct 不接受未知参数: %s", name)
			}
			if behavior == "confusionAttack" && name != "durationActions" && name != "attackPercentModifier" {
				return errors.Errorf("技能 confusionAttack 不接受未知参数: %s", name)
			}
			if behavior == "sleepAttack" && name != "durationActions" && name != "attackPercentModifier" {
				return errors.Errorf("技能 sleepAttack 不接受未知参数: %s", name)
			}
			if behavior == "deepPoisonAttack" && name != "durationActions" && name != "attackPercentModifier" {
				return errors.Errorf("技能 deepPoisonAttack 不接受未知参数: %s", name)
			}
			integerField := behavior == "mightyAttack" && (name == "damageMultiplier" || name == "targetDodgeBonus") ||
				behavior == "poisonAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "stoneAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "confusionAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "sleepAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "deepPoisonAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "chargeAttack" && (name == "chargeRounds" || name == "attackPercentModifier") ||
				behavior == "earthRound" && name == "damagePercentModifier" ||
				behavior == "guardian" && (name == "attackPercentModifier" || name == "defensePercentModifier") ||
				behavior == "noGuard" && (name == "dodgePercent" || name == "counterPercent" || name == "criticalPercent") || statusSpirit || healingSpirit
			integerField = integerField || behavior == "powerBalance" && (name == "attackPercentModifier" || name == "defensePercentModifier") ||
				behavior == "abduct" && name == "loyaltyThreshold"
			if integerField && parameters.Content[field+1].ShortTag() != "!!int" {
				return errors.Errorf("技能 %s.%s 必须为整数", behavior, name)
			}
		}
	}
	type skillEntry SkillEntry
	return node.Decode((*skillEntry)(p))
}

func (p *SkillMightyAttackEntry) check() error {
	if p.DamageMultiplier == nil || p.TargetDodgeBonus == nil {
		return errors.Errorf("一击必杀缺少 mightyAttack.damageMultiplier 或 targetDodgeBonus %v", xruntime.Location())
	}
	// 当前接入整数倍率. 原版COM3低16位保存倍率百分数, 高16位读取为有符号闪避加值.
	if *p.DamageMultiplier < 1 || *p.DamageMultiplier > 655 {
		return errors.Errorf("一击必杀 mightyAttack.damageMultiplier 超出1至655范围: value:%d %v", *p.DamageMultiplier, xruntime.Location())
	}
	if *p.TargetDodgeBonus > 32767 {
		return errors.Errorf("一击必杀 mightyAttack.targetDodgeBonus 超出0至32767范围: value:%d %v", *p.TargetDodgeBonus, xruntime.Location())
	}
	return nil
}

func (p *SkillPoisonAttackEntry) check() error {
	if p.DurationActions == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("中毒攻击缺少 poisonAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版 turn 写入命令高16位并按有符号整数读取, 运行态另加1保留到期解毒那次行动.
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("中毒攻击 poisonAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 0 {
		return errors.Errorf("中毒攻击 poisonAttack.attackPercentModifier 超出-100至0范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillStoneAttackEntry) check() error {
	if p.DurationActions == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("石化攻击缺少 stoneAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版状态攻击把技能turn加1后写入状态表, 最后一次1->0行动仍会被石化阻止.
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("石化攻击 stoneAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 0 {
		return errors.Errorf("石化攻击 stoneAttack.attackPercentModifier 超出-100至0范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillConfusionAttackEntry) check() error {
	if p.DurationActions == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("混乱攻击缺少 confusionAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版混乱会把技能turn加1写入状态表, 归零行动只解除状态并执行原动作.
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("混乱攻击 confusionAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 0 {
		return errors.Errorf("混乱攻击 confusionAttack.attackPercentModifier 超出-100至0范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillSleepAttackEntry) check() error {
	if p.DurationActions == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("催眠攻击缺少 sleepAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版催眠会把技能turn加1写入状态表, 最后一次1->0行动仍会被睡眠阻止.
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("催眠攻击 sleepAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 0 {
		return errors.Errorf("催眠攻击 sleepAttack.attackPercentModifier 超出-100至0范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillDeepPoisonAttackEntry) check() error {
	if p.DurationActions == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("剧毒攻击缺少 deepPoisonAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("剧毒攻击 deepPoisonAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 32767 {
		return errors.Errorf("剧毒攻击 deepPoisonAttack.attackPercentModifier 超出-100至32767范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillChargeAttackEntry) check() error {
	if p.ChargeRounds == nil || p.AttackPercentModifier == nil {
		return errors.Errorf("突击缺少 chargeAttack.chargeRounds 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版蓄力回合范围为1至10, 攻击加值保存在COM3高16位并按有符号整数读取.
	if *p.ChargeRounds < 1 || *p.ChargeRounds > 10 {
		return errors.Errorf("突击 chargeAttack.chargeRounds 超出1至10范围: value:%d %v", *p.ChargeRounds, xruntime.Location())
	}
	if *p.AttackPercentModifier < 0 || *p.AttackPercentModifier > 32767 {
		return errors.Errorf("突击 chargeAttack.attackPercentModifier 超出0至32767范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillEarthRoundEntry) check() error {
	if p.DamagePercentModifier == nil {
		return errors.Errorf("地球一周缺少 earthRound.damagePercentModifier %v", xruntime.Location())
	}
	if *p.DamagePercentModifier < 0 || *p.DamagePercentModifier > 32767 {
		return errors.Errorf("地球一周 earthRound.damagePercentModifier 超出0至32767范围: value:%d %v", *p.DamagePercentModifier, xruntime.Location())
	}
	return nil
}

func (p *SkillGuardianEntry) check() error {
	if p.AttackPercentModifier == nil {
		return errors.Errorf("忠犬缺少 guardian.attackPercentModifier %v", xruntime.Location())
	}
	return nil
}

func (p *SkillNoGuardEntry) check() error {
	if p.DodgePercent == nil || p.CounterPercent == nil || p.CriticalPercent == nil {
		return errors.Errorf("不防守战法缺少 noGuard.dodgePercent、counterPercent 或 criticalPercent %v", xruntime.Location())
	}
	if *p.DodgePercent < 0 || *p.DodgePercent > 32767 {
		return errors.Errorf("不防守战法 noGuard.dodgePercent 超出0至32767范围: value:%d %v", *p.DodgePercent, xruntime.Location())
	}
	if *p.CounterPercent > 255 || *p.CriticalPercent > 255 {
		return errors.Errorf("不防守战法 noGuard.counterPercent或criticalPercent超出0至255范围 %v", xruntime.Location())
	}
	return nil
}

func (p *SkillPowerBalanceEntry) check() error {
	if p.AttackPercentModifier == nil || p.DefensePercentModifier == nil {
		return errors.Errorf("背水之战缺少 powerBalance.attackPercentModifier 或 defensePercentModifier %v", xruntime.Location())
	}
	return nil
}

func newSkillConfig() *SkillConfig {
	return &SkillConfig{
		MapMgr: xmap.NewMapMgr[uint32, *SkillEntry](),
	}
}

func (p *SkillConfig) load(dir string) error {
	var root struct {
		Skill []*SkillEntry `yaml:"skill"`
	}
	if err := loadYAMLFile(dir, FileSkill, &root); err != nil {
		return err
	}
	return p.configure(root.Skill)
}

func (p *SkillConfig) configure(entries []*SkillEntry) error {
	for _, skill := range entries {
		if skill.ID == nil {
			return errors.Errorf("技能缺少 id %v", xruntime.Location())
		}
		if !isSkillID(*skill.ID) {
			return errors.Errorf("技能ID超出范围: %d %v", *skill.ID, xruntime.Location())
		}
		if len(skill.UsableBy) == 0 {
			return errors.Errorf("技能 usableBy 至少需要character或pet: ID:%d %v", *skill.ID, xruntime.Location())
		}
		usableBy := make(map[string]struct{}, len(skill.UsableBy))
		for _, subject := range skill.UsableBy {
			if subject != "character" && subject != "pet" {
				return errors.Errorf("技能 usableBy 包含未知单位类型: ID:%d value:%s %v", *skill.ID, subject, xruntime.Location())
			}
			if _, exists := usableBy[subject]; exists {
				return errors.Errorf("技能 usableBy 包含重复单位类型: ID:%d value:%s %v", *skill.ID, subject, xruntime.Location())
			}
			usableBy[subject] = struct{}{}
		}
		behaviorCount := 0
		if skill.ContinuationAttack != nil {
			behaviorCount++
			if err := skill.ContinuationAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.MightyAttack != nil {
			behaviorCount++
			if err := skill.MightyAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.PoisonAttack != nil {
			behaviorCount++
			if err := skill.PoisonAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.StoneAttack != nil {
			behaviorCount++
			if err := skill.StoneAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("石化攻击只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.ConfusionAttack != nil {
			behaviorCount++
			if err := skill.ConfusionAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("混乱攻击只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.SleepAttack != nil {
			behaviorCount++
			if err := skill.SleepAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("催眠攻击只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.DeepPoisonAttack != nil {
			behaviorCount++
			if err := skill.DeepPoisonAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("剧毒攻击只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.ChargeAttack != nil {
			behaviorCount++
			if err := skill.ChargeAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.EarthRound != nil {
			behaviorCount++
			if err := skill.EarthRound.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("地球一周技能只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.Guardian != nil {
			behaviorCount++
			if err := skill.Guardian.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.NoGuard != nil {
			behaviorCount++
			if err := skill.NoGuard.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.PowerBalance != nil {
			behaviorCount++
			if err := skill.PowerBalance.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.ShowMercy != nil {
			behaviorCount++
		}
		if skill.Abduct != nil {
			behaviorCount++
			if err := skill.Abduct.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("旅程伙伴技能只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		if skill.Processing != nil {
			behaviorCount++
			if !skill.CanBeUsedBy("pet") || skill.CanBeUsedBy("character") || skill.MPCost != nil || skill.TargetScope != "" {
				return errors.Errorf("加工技能只允许pet使用且不接受mpCost或targetScope: ID:%d %v", *skill.ID, xruntime.Location())
			}
		}
		statusSpirits := []struct {
			name     string
			entry    *SkillPoisonSpiritEntry
			statusID uint32
		}{
			{name: "猛毒/毒雾精灵", entry: skill.PoisonSpirit, statusID: 1},
			{name: "硬化/石化精灵", entry: (*SkillPoisonSpiritEntry)(skill.StoneSpirit), statusID: 4},
			{name: "混乱/混迷精灵", entry: (*SkillPoisonSpiritEntry)(skill.ConfusionSpirit), statusID: 6},
			{name: "酒的/酩酊精灵", entry: (*SkillPoisonSpiritEntry)(skill.DrunkSpirit), statusID: 5},
			{name: "睡眠/昏睡精灵", entry: (*SkillPoisonSpiritEntry)(skill.SleepSpirit), statusID: 3},
		}
		for _, spirit := range statusSpirits {
			if spirit.entry == nil {
				continue
			}
			behaviorCount++
			if skill.MPCost == nil {
				return errors.Errorf("异常精灵技能缺少mpCost: ID:%d %v", *skill.ID, xruntime.Location())
			}
			if skill.TargetScope != "singleOpponent" && skill.TargetScope != "opponentCamp" {
				return errors.Errorf("异常精灵技能targetScope必须为singleOpponent或opponentCamp: ID:%d value:%s %v", *skill.ID, skill.TargetScope, xruntime.Location())
			}
			if err := checkStatusSpirit(spirit.name, spirit.entry, spirit.statusID); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		healingSpirits := []struct {
			name        string
			entry       *SkillHealingSpiritEntry
			targetScope string
		}{
			{name: "治愈的精灵", entry: skill.HealingSpirit, targetScope: "self"},
			{name: "滋润的精灵", entry: (*SkillHealingSpiritEntry)(skill.MoistureSpirit), targetScope: "singleAlly"},
			{name: "恩惠的精灵", entry: (*SkillHealingSpiritEntry)(skill.GraceSpirit), targetScope: "allyCamp"},
		}
		for _, spirit := range healingSpirits {
			if spirit.entry == nil {
				continue
			}
			behaviorCount++
			if skill.MPCost == nil {
				return errors.Errorf("治疗精灵技能缺少mpCost: ID:%d %v", *skill.ID, xruntime.Location())
			}
			if skill.TargetScope != spirit.targetScope {
				return errors.Errorf("%s targetScope必须为%s: ID:%d value:%s %v", spirit.name, spirit.targetScope, *skill.ID, skill.TargetScope, xruntime.Location())
			}
			if err := checkHealingSpirit(spirit.name, spirit.entry); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if behaviorCount > 1 {
			return errors.Errorf("技能行为参数块必须互斥: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if !p.AddIfNotExist(*skill.ID, skill) {
			return errors.Errorf("技能ID重复: %d %v", *skill.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *SkillConfig) check() error {
	return nil
}

func (p *SkillConfig) assemble() error {
	return nil
}
