package gameconfig

import (
	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type ExpConfig struct {
	*xmap.MapMgr[uint32, *LevelEntry]
}

type LevelEntry struct {
	// Level 来自 exp.yaml 的 levels map key, 必须完整覆盖协议等级范围且连续.
	Level *uint32
	// MinExp 是 server 在加载 exp.yaml 时派生的本等级最小累计经验, 1 级固定为0, 其他等级为上一等级 MaxExp+1.
	MinExp *uint32
	// MaxExp 来自 levels.<level>.max, 表示本等级最大累计经验, 必须非负并随等级严格递增.
	MaxExp *uint32 `yaml:"max"`
}

func (p *ExpConfig) GetLevel(totalExp uint32) (uint32, error) {
	for level := uint32(pb.LevelRange_LevelRange_Min); level <= uint32(pb.LevelRange_LevelRange_Max); level++ {
		entry := p.Get(level)
		if entry == nil {
			return 0, errors.Errorf("经验等级不存在: %d %v", level, xruntime.Location())
		}
		if totalExp <= *entry.MaxExp {
			return *entry.Level, nil
		}
	}
	return uint32(pb.LevelRange_LevelRange_Max), nil
}

func (p *ExpConfig) GetNextLevelTotalExp(totalExp uint32) (uint32, bool, error) {
	level, err := p.GetLevel(totalExp)
	if err != nil {
		return 0, false, err
	}
	if level >= uint32(pb.LevelRange_LevelRange_Max) {
		return 0, false, nil
	}
	nextLevelTotalExp, err := p.GetLevelMinExp(level + 1)
	if err != nil {
		return 0, false, err
	}
	return nextLevelTotalExp, true, nil
}

func (p *ExpConfig) GetLevelMinExp(level uint32) (uint32, error) {
	if level < uint32(pb.LevelRange_LevelRange_Min) || level > uint32(pb.LevelRange_LevelRange_Max) {
		return 0, errors.Errorf("经验等级不存在: %d %v", level, xruntime.Location())
	}
	entry := p.Get(level)
	if entry == nil {
		return 0, errors.Errorf("经验等级不存在: %d %v", level, xruntime.Location())
	}
	return *entry.MinExp, nil
}

func (p *ExpConfig) IsMaxLevel(totalExp uint32) (bool, error) {
	level, err := p.GetLevel(totalExp)
	if err != nil {
		return false, err
	}
	return level >= uint32(pb.LevelRange_LevelRange_Max), nil
}

func newExpConfig() *ExpConfig {
	return &ExpConfig{
		MapMgr: xmap.NewMapMgr[uint32, *LevelEntry](),
	}
}

func (p *ExpConfig) load(dir string) error {
	var root struct {
		Levels map[uint32]*LevelEntry `yaml:"levels"`
	}
	if err := loadYAMLFile(dir, FileExp, &root); err != nil {
		return err
	}
	return p.configure(root.Levels)
}

func (p *ExpConfig) configure(levels map[uint32]*LevelEntry) error {
	minLevel := uint32(pb.LevelRange_LevelRange_Min)
	maxLevel := uint32(pb.LevelRange_LevelRange_Max)
	for level, entry := range levels {
		if level < minLevel || level > maxLevel {
			return errors.Errorf("经验等级超出协议范围: level:%d expected:[%d,%d] %v", level, minLevel, maxLevel, xruntime.Location())
		}
		p.Add(level, entry)
	}

	var previous *LevelEntry
	for level := minLevel; level <= maxLevel; level++ {
		entry, ok := p.Find(level)
		if !ok {
			return errors.Errorf("经验等级必须连续覆盖协议范围: missing:%d %v", level, xruntime.Location())
		}
		if entry == nil {
			return errors.Errorf("经验等级配置不能为空: level:%d %v", level, xruntime.Location())
		}
		if entry.MaxExp == nil {
			return errors.Errorf("配置缺少必填字段: %s.levels.%d.max %v", FileExp, level, xruntime.Location())
		}
		levelValue := level
		entry.Level = &levelValue
		if previous == nil {
			minExp := uint32(0)
			entry.MinExp = &minExp
		} else {
			if *entry.MaxExp <= *previous.MaxExp {
				return errors.Errorf("经验等级 max 必须严格递增: level:%d max:%d previous_max:%d %v", level, *entry.MaxExp, *previous.MaxExp, xruntime.Location())
			}
			minExp := *previous.MaxExp + 1
			entry.MinExp = &minExp
		}
		if *entry.MinExp > *entry.MaxExp {
			return errors.Errorf("经验等级区间不能反向: level:%d min:%d max:%d %v", level, *entry.MinExp, *entry.MaxExp, xruntime.Location())
		}
		previous = entry
	}
	return nil
}

func (p *ExpConfig) check() error {
	return nil
}

func (p *ExpConfig) assemble() error {
	return nil
}
