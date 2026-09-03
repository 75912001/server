package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type SkillConfig struct {
	*xmap.MapMgr[uint32, *SkillEntry]
}

type SkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议技能共用资源ID段内, 并且在 skill.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 skill[].name, 保留技能显示名称文本, server 不解析客户端资源.
	Name *string `yaml:"name"`
	// Description 来自 skill[].description, 保留技能说明文本, 可用于查询、日志或后续业务展示下发.
	Description *string `yaml:"description"`
	// Cost 非 nil 表示该技能允许宠物学习, 数值为原版基础石币价格; 0是合法的免费学习价格.
	Cost *uint64 `yaml:"cost"`
	// ContinuationAttack 非 nil 表示连续攻击, 段数由服务端配置决定.
	ContinuationAttack *SkillContinuationAttackEntry `yaml:"continuationAttack"`
	// MightyAttack 非 nil 表示一击必杀, 只修正本次主动攻击的最终伤害和目标闪避.
	MightyAttack *SkillMightyAttackEntry `yaml:"mightyAttack"`
	// PoisonAttack 非 nil 表示物理攻击附加普通中毒, 毒伤由目标基础四维决定.
	PoisonAttack *SkillPoisonAttackEntry `yaml:"poisonAttack"`
	// ChargeAttack 非 nil 表示先蓄力若干回合, 再以提高后的基础攻击力执行一次物理攻击.
	ChargeAttack *SkillChargeAttackEntry `yaml:"chargeAttack"`
	// ShowMercy 非 nil 表示普通单段物理攻击只扣到目标剩1HP, 不赋予持续保命状态.
	ShowMercy *SkillShowMercyEntry `yaml:"showMercy"`
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

type SkillChargeAttackEntry struct {
	ChargeRounds          *uint32 `yaml:"chargeRounds"`
	AttackPercentModifier *int32  `yaml:"attackPercentModifier"`
}

// SkillShowMercyEntry是固定留1HP的无参数标记, YAML只接受空对象.
type SkillShowMercyEntry struct{}

// UnmarshalYAML拒绝技能参数对象的空值和非整数参数, 避免YAML解码静默截断小数.
func (p *SkillEntry) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		behavior := node.Content[index].Value
		if behavior != "mightyAttack" && behavior != "poisonAttack" && behavior != "chargeAttack" && behavior != "showMercy" {
			continue
		}
		parameters := node.Content[index+1]
		if parameters.Kind == yaml.AliasNode {
			parameters = parameters.Alias
		}
		if parameters.Kind != yaml.MappingNode {
			return errors.Errorf("技能 %s 必须为对象", behavior)
		}
		if behavior == "showMercy" && len(parameters.Content) != 0 {
			return errors.New("技能 showMercy 必须为空对象, 不接受参数")
		}
		for field := 0; field+1 < len(parameters.Content); field += 2 {
			name := parameters.Content[field].Value
			integerField := behavior == "mightyAttack" && (name == "damageMultiplier" || name == "targetDodgeBonus") ||
				behavior == "poisonAttack" && (name == "durationActions" || name == "attackPercentModifier") ||
				behavior == "chargeAttack" && (name == "chargeRounds" || name == "attackPercentModifier")
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
		return errors.Errorf("猛毒攻击缺少 poisonAttack.durationActions 或 attackPercentModifier %v", xruntime.Location())
	}
	// 原版 turn 写入命令高16位并按有符号整数读取, 运行态另加1保留到期解毒那次行动.
	if *p.DurationActions < 1 || *p.DurationActions > 32767 {
		return errors.Errorf("猛毒攻击 poisonAttack.durationActions 超出1至32767范围: value:%d %v", *p.DurationActions, xruntime.Location())
	}
	if *p.AttackPercentModifier < -100 || *p.AttackPercentModifier > 0 {
		return errors.Errorf("猛毒攻击 poisonAttack.attackPercentModifier 超出-100至0范围: value:%d %v", *p.AttackPercentModifier, xruntime.Location())
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
		if skill.Name == nil {
			return errors.Errorf("技能缺少 name: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Name) == "" {
			return errors.Errorf("技能 name 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if skill.Description == nil {
			return errors.Errorf("技能缺少 description: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Description) == "" {
			return errors.Errorf("技能 description 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
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
		if skill.ChargeAttack != nil {
			behaviorCount++
			if err := skill.ChargeAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.ShowMercy != nil {
			behaviorCount++
		}
		if behaviorCount > 1 {
			return errors.Errorf("技能 continuationAttack, mightyAttack, poisonAttack, chargeAttack 和 showMercy 互斥: ID:%d %v", *skill.ID, xruntime.Location())
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
