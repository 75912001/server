package gameconfig

import (
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
	pet := GGameConfig.Pet.Get(petID)
	if pet == nil {
		return 0, errors.Errorf("生成怪物经验失败, 宠物不存在: id:%d %v", petID, xruntime.Location())
	}
	baseExp, ok := p.Find(lv)
	if !ok {
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

	exp := baseExp + (rankBonusHundredths+alphaHundredths)*lv/100
	if exp < 1 {
		return 1, nil
	}
	return exp, nil
}
