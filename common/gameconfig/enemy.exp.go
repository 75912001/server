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
		if level < uint32(pb.LevelRange_LevelRange_Min) || uint32(pb.LevelRange_LevelRange_Max) < level {
			err = errors.Errorf("敌人基础经验等级超出范围: level:%d expected:[%d,%d] %v",
				level, pb.LevelRange_LevelRange_Min, pb.LevelRange_LevelRange_Max, xruntime.Location())
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	for lv := uint32(pb.LevelRange_LevelRange_Min); lv <= uint32(pb.LevelRange_LevelRange_Max); lv++ {
		if !p.IsExist(lv) {
			return errors.Errorf("敌人基础经验等级配置不连续: level:%d expected:[%d,%d] %v", lv, pb.LevelRange_LevelRange_Min, lv, xruntime.Location())
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

func (p *EnemyExpConfig) GenerateEnemyExp(petID uint32, lv uint32) (uint32, error) {
	if GGameConfig == nil || GGameConfig.Pet == nil {
		return 0, errors.Errorf("生成怪物经验失败, 宠物配置未加载 %v", xruntime.Location())
	}
	pet := GGameConfig.Pet.Get(petID)
	if pet == nil {
		return 0, errors.Errorf("生成怪物经验失败, 宠物不存在: id:%d %v", petID, xruntime.Location())
	}
	baseExp, ok := p.Find(lv)
	if !ok {
		return 0, errors.Errorf("敌人基础经验等级不存在: level:%d", lv)
	}
	if pet.Growth == nil || pet.Attribute == nil ||
		pet.Growth.BaseVital == nil || pet.Growth.BaseStr == nil ||
		pet.Growth.BaseTough == nil || pet.Growth.BaseDex == nil ||
		pet.Attribute.Critical == nil || pet.Attribute.Counter == nil ||
		pet.Attribute.Get == nil || pet.Attribute.PoisonResist == nil ||
		pet.Attribute.ParalysisResist == nil || pet.Attribute.SleepResist == nil ||
		pet.Attribute.StoneResist == nil || pet.Attribute.DrunkResist == nil ||
		pet.Attribute.ConfusionResist == nil || pet.Attribute.Rare == nil {
		return 0, errors.Errorf("生成怪物经验失败, 宠物成长或属性配置不完整: id:%d %v",
			petID, xruntime.Location())
	}

	baseSum := uint64(*pet.Growth.BaseVital) +
		uint64(*pet.Growth.BaseStr) +
		uint64(*pet.Growth.BaseTough) +
		uint64(*pet.Growth.BaseDex)
	rank := petRankFromBaseSum(baseSum)
	rankBonus := [...]float32{2.5, 2.0, 1.5, 1.0, 0.5, 0.0}[rank]

	// ENEMY_getExp先让整数状态总和除以double字面量100.0, 与Rare相加后
	// 赋给float alpha; 随后的ranknum、alpha、level和基础经验运算都在
	// C float精度中逐步进行, 最后赋给C int并向零截断. 这里不能改写成
	// “百分整数先乘等级再除100”: float32在整数边界附近的舍入可能使最终
	// 经验相差1, 负alpha还会被无符号转换放大成完全错误的巨值.
	attributeSum := int64(*pet.Attribute.Critical) +
		int64(*pet.Attribute.Counter) +
		int64(*pet.Attribute.Get) +
		int64(*pet.Attribute.PoisonResist) +
		int64(*pet.Attribute.ParalysisResist) +
		int64(*pet.Attribute.SleepResist) +
		int64(*pet.Attribute.StoneResist) +
		int64(*pet.Attribute.DrunkResist) +
		int64(*pet.Attribute.ConfusionResist)
	alpha := float32(float64(attributeSum)/100.0 + float64(*pet.Attribute.Rare))
	expFloat := float32(baseExp) + (rankBonus+alpha)*float32(lv)
	if expFloat < 1 {
		return 1, nil
	}
	if expFloat > float32(math.MaxInt32) {
		return 0, errors.Errorf("生成怪物经验超出C int范围: pet:%d level:%d value:%v %v",
			petID, lv, expFloat, xruntime.Location())
	}
	return uint32(expFloat), nil
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
