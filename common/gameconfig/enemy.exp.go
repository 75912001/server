package gameconfig

import (
	"sort"
	"strconv"
)

func newEnemyExpConfig() *EnemyExpConfig {
	return &EnemyExpConfig{
		byLevel: map[uint32]uint32{},
	}
}

func (e *EnemyExpConfig) load(dir string) error {
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

	levels := make([]uint32, 0, len(values))
	for rawLevel, valueNode := range values {
		level, err := strconv.ParseUint(rawLevel, 10, 32)
		if err != nil {
			return configError("敌人基础经验等级 key 必须是整数: %s", rawLevel)
		}
		if level <= 0 {
			return configError("敌人基础经验等级必须大于0: level:%d", level)
		}
		lv := uint32(level)
		if _, ok := e.byLevel[lv]; ok {
			return configError("敌人基础经验等级重复: %d", level)
		}
		baseExp, err := nonNegativeIntScalar(valueNode, FileEnemyExp+".enemyExp."+rawLevel)
		if err != nil {
			return err
		}
		baseExpValue := uint32(baseExp)
		if int(baseExpValue) != baseExp {
			return configError("敌人基础经验值超出范围: level:%d value:%d", level, baseExp)
		}
		e.byLevel[lv] = baseExpValue
		levels = append(levels, lv)
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i] < levels[j]
	})
	e.levels = levels
	return nil
}

func (e *EnemyExpConfig) check() error {
	if e == nil || len(e.byLevel) == 0 {
		return configError("敌人基础经验配置尚未加载")
	}
	return nil
}

func (e *EnemyExpConfig) assemble() error {
	return nil
}

func (e *EnemyExpConfig) getBaseExp(level uint32) (uint32, error) {
	if e == nil || len(e.byLevel) == 0 {
		return 0, configError("敌人基础经验配置尚未加载")
	}
	baseExp, ok := e.byLevel[level]
	if !ok {
		return 0, configError("敌人基础经验等级不存在: %d", level)
	}
	return baseExp, nil
}

func (m *Manager) GenerateEnemyExp(id uint32, lv uint32) (uint32, error) {
	if m == nil || m.Pet == nil || m.EnemyExp == nil {
		return 0, configError("游戏配置尚未加载")
	}
	petID := int(id)
	if uint32(petID) != id {
		return 0, configError("生成怪物经验失败, 宠物不存在: id:%d", id)
	}
	pet := m.Pet.GetByID(petID)
	if pet == nil {
		return 0, configError("生成怪物经验失败, 宠物不存在: id:%d", id)
	}
	baseExp, err := m.EnemyExp.getBaseExp(lv)
	if err != nil {
		return 0, err
	}
	baseExpValue := int64(baseExp)
	level := int64(lv)

	growth := pet.Growth
	attribute := pet.Attribute
	baseSum := int64(growth.BaseVital) + int64(growth.BaseStr) + int64(growth.BaseTough) + int64(growth.BaseDex)
	// rank 加成是 2.5, 2.0 等小数, 这里统一乘以 100 用整数计算.
	rankBonusHundredths := int64(0)
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
	alphaHundredths := int64(attribute.Critical) +
		int64(attribute.Counter) +
		int64(attribute.Get) +
		int64(attribute.PoisonResist) +
		int64(attribute.ParalysisResist) +
		int64(attribute.SleepResist) +
		int64(attribute.StoneResist) +
		int64(attribute.DrunkResist) +
		int64(attribute.ConfusionResist) +
		int64(attribute.Rare)*100

	exp := baseExpValue + (rankBonusHundredths+alphaHundredths)*level/100
	if exp < 1 {
		return 1, nil
	}
	if exp > int64(^uint32(0)) {
		return 0, configError("生成怪物经验失败, 经验值超出范围: id:%d level:%d value:%d", id, lv, exp)
	}
	return uint32(exp), nil
}
