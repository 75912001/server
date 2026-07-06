package gameconfig

import (
	pb "server/proto/pb"
)

type EnemyExpConfig struct {
	EnemyExp map[uint32]*uint32 `yaml:"enemyExp"`
}

func newEnemyExpConfig() *EnemyExpConfig {
	return &EnemyExpConfig{}
}

func (p *EnemyExpConfig) load(dir string) error {
	if err := loadYAMLFile(dir, FileEnemyExp, p); err != nil {
		return err
	}
	return p.configure()
}

func (p *EnemyExpConfig) configure() error {
	if len(p.EnemyExp) == 0 {
		return configError("敌人基础经验配置中没有解析到 enemyExp 数据: %s", FileEnemyExp)
	}
	for level, value := range p.EnemyExp {
		if level < uint32(pb.LevelRange_LevelRange_Min) || uint32(pb.LevelRange_LevelRange_Max) < level {
			return configError("敌人基础经验等级超出范围: level:%d expected:[%d,%d]", level, pb.LevelRange_LevelRange_Min, pb.LevelRange_LevelRange_Max)
		}
		if value == nil {
			return configError("敌人基础经验等级配置不能为空: level:%d", level)
		}
	}
	for lv := uint32(pb.LevelRange_LevelRange_Min); lv <= uint32(pb.LevelRange_LevelRange_Max); lv++ {
		_, ok := p.EnemyExp[lv]
		if !ok { // 没有
			return configError("敌人基础经验等级配置不连续: level:%d", lv)
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

func (p *Manager) GenerateEnemyExp(petID uint32, lv uint32) (uint32, error) {
	pet := p.Pet.GetByID(int(petID))
	baseExp, ok := p.EnemyExp.EnemyExp[lv]
	if !ok { // 没有
		return 0, configError("敌人基础经验等级不存在: level:%d", lv)
	}
	baseSum := *pet.Growth.BaseVital + *pet.Growth.BaseStr + *pet.Growth.BaseTough + *pet.Growth.BaseDex
	// rank 加成是 2.5, 2.0 等小数, 这里统一乘以 100 用整数计算.
	rankBonusHundredths := uint32(0)
	switch {
	case baseSum >= 100:
		rankBonusHundredths = 250
	case baseSum >= 95:
		rankBonusHundredths = 200
	case baseSum >= 90:
		rankBonusHundredths = 150
	case baseSum >= 85:
		rankBonusHundredths = 100
	case baseSum >= 80:
		rankBonusHundredths = 50
	}
	alphaHundredths := *pet.Attribute.Critical +
		*pet.Attribute.Counter +
		*pet.Attribute.Get +
		*pet.Attribute.PoisonResist +
		*pet.Attribute.ParalysisResist +
		*pet.Attribute.SleepResist +
		*pet.Attribute.StoneResist +
		*pet.Attribute.DrunkResist +
		*pet.Attribute.ConfusionResist +
		*pet.Attribute.Rare*100

	exp := *baseExp + (rankBonusHundredths+alphaHundredths)*lv/100
	if exp < 1 {
		return 1, nil
	}
	return exp, nil
}
