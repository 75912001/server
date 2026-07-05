package gameconfig

import (
	"sort"
	"strconv"

	pb "server/proto/pb"
)

func newEnemyExpConfig() *EnemyExpConfig {
	return &EnemyExpConfig{
		byLevel: map[uint32]uint32{},
	}
}

func (p *EnemyExpConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FileEnemyExp)
	if err != nil {
		return err
	}
	expNode, err := requireKey(root, "enemyExp", FileEnemyExp)
	if err != nil {
		return err
	}
	values, err := requireMap(expNode, FileEnemyExp+".enemyExp")
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return configError("敌人基础经验配置中没有解析到 enemyExp 数据: %s", FileEnemyExp)
	}

	minLevel := uint32(pb.LevelRange_LevelRange_Min)
	maxLevel := uint32(pb.LevelRange_LevelRange_Max)
	levels := make([]uint32, 0, len(values))
	for rawLevel, valueNode := range values {
		level, err := strconv.ParseUint(rawLevel, 10, 32)
		if err != nil {
			return configError("敌人基础经验等级 key 必须是整数: %s", rawLevel)
		}
		lv := uint32(level)
		if lv < minLevel || lv > maxLevel {
			return configError("敌人基础经验等级超出范围: level:%d expected:[%d,%d]", lv, minLevel, maxLevel)
		}
		if _, ok := p.byLevel[lv]; ok {
			return configError("敌人基础经验等级重复: %d", level)
		}
		baseExpValue, err := uint32Scalar(valueNode, FileEnemyExp+".enemyExp."+rawLevel)
		if err != nil {
			return err
		}
		p.byLevel[lv] = baseExpValue
		levels = append(levels, lv)
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i] < levels[j]
	})
	if len(levels) != int(maxLevel-minLevel+1) {
		return configError("敌人基础经验等级必须连续覆盖[%d,%d]: count:%d", minLevel, maxLevel, len(levels))
	}
	for index, level := range levels {
		expected := minLevel + uint32(index)
		if level != expected {
			return configError("敌人基础经验等级必须连续覆盖[%d,%d]: missing:%d got:%d", minLevel, maxLevel, expected, level)
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

func (p *EnemyExpConfig) getBaseExp(level uint32) (uint32, error) {
	baseExp, ok := p.byLevel[level]
	if !ok {
		return 0, configError("敌人基础经验等级不存在: %d", level)
	}
	return baseExp, nil
}

func (p *Manager) GenerateEnemyExp(petID uint32, lv uint32) (uint32, error) {
	if p.Pet == nil || p.EnemyExp == nil {
		return 0, configError("游戏配置尚未加载")
	}
	pet := p.Pet.GetByID(petID)
	if pet == nil {
		return 0, configError("生成怪物经验失败, 宠物不存在: id:%d", petID)
	}
	baseExp, err := p.EnemyExp.getBaseExp(lv)
	if err != nil {
		return 0, err
	}

	baseSum := pet.Growth.BaseVital + pet.Growth.BaseStr + pet.Growth.BaseTough + pet.Growth.BaseDex
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
	alphaHundredths := pet.Attribute.Critical +
		pet.Attribute.Counter +
		pet.Attribute.Get +
		pet.Attribute.PoisonResist +
		pet.Attribute.ParalysisResist +
		pet.Attribute.SleepResist +
		pet.Attribute.StoneResist +
		pet.Attribute.DrunkResist +
		pet.Attribute.ConfusionResist +
		pet.Attribute.Rare*100

	exp := baseExp + (rankBonusHundredths+alphaHundredths)*lv/100
	if exp < 1 {
		return 1, nil
	}
	return exp, nil
}
