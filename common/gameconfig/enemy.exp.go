package gameconfig

import (
	"math"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type EnemyExpConfig struct {
	*xmap.MapMgr[uint32, uint32]
}

func newEnemyExpConfig() *EnemyExpConfig {
	return &EnemyExpConfig{
		MapMgr: xmap.NewMapMgr[uint32, uint32](),
	}
}

func (p *EnemyExpConfig) load(dir string) error {
	var root struct {
		EnemyExp map[uint32]*uint32 `yaml:"enemyExp"`
	}
	if err := loadYAMLFile(dir, FileEnemyExp, &root); err != nil {
		return err
	}
	enemyExp := xmap.NewMapMgr[uint32, uint32]()
	for level, value := range root.EnemyExp {
		if value == nil {
			return errors.Errorf("敌人基础经验不能为空: level:%d %v", level, xruntime.Location())
		}
		enemyExp.Add(level, *value)
	}
	p.MapMgr = enemyExp
	return p.configure()
}

func (p *EnemyExpConfig) configure() error {
	var err error
	p.Foreach(func(level uint32, value uint32) bool {
		if level < uint32(pb.Constants_Constants_Level_Min) || uint32(pb.Constants_Constants_Level_Max) < level {
			err = errors.Errorf("敌人基础经验等级超出范围: level:%d expected:[%d,%d] %v",
				level, pb.Constants_Constants_Level_Min, pb.Constants_Constants_Level_Max, xruntime.Location())
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	for lv := uint32(pb.Constants_Constants_Level_Min); lv <= uint32(pb.Constants_Constants_Level_Max); lv++ {
		if !p.IsExist(lv) {
			return errors.Errorf("敌人基础经验等级配置不连续: level:%d expected:[%d,%d] %v", lv, pb.Constants_Constants_Level_Min, lv, xruntime.Location())
		}
	}
	return nil
}

func (p *EnemyExpConfig) check() error {
	return nil
}

func (p *EnemyExpConfig) assemble() error {
	return nil
}

func (p *EnemyExpConfig) GenerateEnemyExp(growthAttributeID uint32, lv uint32) (uint32, error) {
	if GGameConfig == nil || GGameConfig.GrowthAttribute == nil {
		return 0, errors.Errorf("生成怪物经验失败, 成长属性配置未加载 %v", xruntime.Location())
	}
	growthAttribute := GGameConfig.GrowthAttribute.Get(growthAttributeID)
	if growthAttribute == nil {
		return 0, errors.Errorf("生成怪物经验失败, 成长属性不存在: id:%d %v", growthAttributeID, xruntime.Location())
	}
	baseExp, ok := p.Find(lv)
	if !ok {
		return 0, errors.Errorf("敌人基础经验等级不存在: level:%d", lv)
	}
	growth := growthAttribute.Growth
	attribute := growthAttribute.Attribute
	if growth == nil || attribute == nil ||
		growth.BaseVital == nil || growth.BaseStr == nil ||
		growth.BaseTough == nil || growth.BaseDex == nil ||
		attribute.Critical == nil || attribute.Counter == nil ||
		attribute.Get == nil || attribute.PoisonResist == nil ||
		attribute.ParalysisResist == nil || attribute.SleepResist == nil ||
		attribute.StoneResist == nil || attribute.DrunkResist == nil ||
		attribute.ConfusionResist == nil || attribute.Rare == nil {
		return 0, errors.Errorf("生成怪物经验失败, 成长属性配置不完整: id:%d %v",
			growthAttributeID, xruntime.Location())
	}

	baseSum := uint64(*growth.BaseVital) +
		uint64(*growth.BaseStr) +
		uint64(*growth.BaseTough) +
		uint64(*growth.BaseDex)
	rank := petRankFromBaseSum(baseSum)
	rankBonus := [...]float32{2.5, 2.0, 1.5, 1.0, 0.5, 0.0}[rank]

	// ENEMY_getExp先让整数状态总和除以double字面量100.0, 与Rare相加后
	// 赋给float alpha; 随后的ranknum、alpha、level和基础经验运算都在
	// C float精度中逐步进行, 最后赋给C int并向零截断. 这里不能改写成
	// “百分整数先乘等级再除100”: float32在整数边界附近的舍入可能使最终
	// 经验相差1, 负alpha还会被无符号转换放大成完全错误的巨值.
	attributeSum := int64(*attribute.Critical) +
		int64(*attribute.Counter) +
		int64(*attribute.Get) +
		int64(*attribute.PoisonResist) +
		int64(*attribute.ParalysisResist) +
		int64(*attribute.SleepResist) +
		int64(*attribute.StoneResist) +
		int64(*attribute.DrunkResist) +
		int64(*attribute.ConfusionResist)
	alpha := float32(float64(attributeSum)/100.0 + float64(*attribute.Rare))
	expFloat := float32(baseExp) + (rankBonus+alpha)*float32(lv)
	if expFloat < 1 {
		return 1, nil
	}
	if expFloat > float32(math.MaxInt32) {
		return 0, errors.Errorf("生成怪物经验超出C int范围: growthAttribute:%d level:%d value:%v %v",
			growthAttributeID, lv, expFloat, xruntime.Location())
	}
	return uint32(expFloat), nil
}

// GenerateEnemyDefeatExperience一次完成“生成怪物经验+按等级差衰减”. // todo menglc [优化] 直接使用该函数, GenerateEnemyExp / CalculateEnemyDefeatExperience 可以改成内部的函数, 或者直接原地展开, 移除那两个函数.
func (p *EnemyExpConfig) GenerateEnemyDefeatExperience(growthAttributeID uint32, enemyLevel uint32, attackerLevel uint32) (uint64, error) {
	enemyExp, err := p.GenerateEnemyExp(growthAttributeID, enemyLevel)
	if err != nil {
		return 0, err
	}
	return CalculateEnemyDefeatExperience(enemyExp, attackerLevel, enemyLevel), nil
}

// CalculateEnemyDefeatExperience复刻BATTLE_AddExpItem对一只刚死亡敌人的等级差经验衰减.
//
// 规则按每只敌人、每名实际攻击参与者独立执行:
//   - 攻击者等级不高于敌人等级+5时取得敌人完整CHAR_EXP.
//   - 超过5级后按(20-等级差)/15向下取整.
//   - 等级差达到20及以上时固定取得1点; 衰减结果不足1也固定为1.
//
// 每只敌人的衰减结果会直接累加到本场经验, 战斗结束时不再应用额外倍率.
func CalculateEnemyDefeatExperience(enemyExp uint32, attackerLevel uint32, enemyLevel uint32) uint64 {
	const (
		fullExperienceMaximumLevelDifference int64 = 5
		experienceReductionDivisor           int64 = 15
	)
	levelDifference := int64(attackerLevel) - int64(enemyLevel)
	if levelDifference <= fullExperienceMaximumLevelDifference {
		return uint64(enemyExp)
	}
	reductionNumerator := fullExperienceMaximumLevelDifference +
		experienceReductionDivisor -
		levelDifference
	if reductionNumerator > experienceReductionDivisor {
		reductionNumerator = experienceReductionDivisor
	}
	if reductionNumerator <= 0 {
		return 1
	}
	experience := uint64(enemyExp) * uint64(reductionNumerator) /
		uint64(experienceReductionDivisor)
	if experience < 1 {
		return 1
	}
	return experience
}
