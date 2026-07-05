package gameconfig

import (
	"sort"
	"strconv"

	pb "server/proto/pb"
)

func newExpConfig() *ExpConfig {
	return &ExpConfig{
		byLevel: map[uint32]*LevelEntry{},
	}
}

func (e *ExpConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FileExp)
	if err != nil {
		return err
	}
	levelsNode, err := requireKey(root, "levels", FileExp)
	if err != nil {
		return err
	}
	levels, err := requireMap(levelsNode, FileExp+".levels")
	if err != nil {
		return err
	}
	if len(levels) == 0 {
		return configError("经验配置中没有解析到 levels 数据: %s", FileExp)
	}

	minLevel := int(pb.LevelRange_LevelRange_Min)
	maxLevel := int(pb.LevelRange_LevelRange_Max)
	levelNumbers := make([]int, 0, len(levels))
	for rawLevel, levelNode := range levels {
		level, err := strconv.Atoi(rawLevel)
		if err != nil {
			return configError("经验等级 key 必须是整数: %s", rawLevel)
		}
		if level < minLevel || level > maxLevel {
			return configError("经验等级超出协议范围: level:%d range:[%d,%d]", level, minLevel, maxLevel)
		}
		levelKey := uint32(level)
		if int(levelKey) != level {
			return configError("经验等级超出范围: %d", level)
		}
		if _, ok := e.byLevel[levelKey]; ok {
			return configError("经验等级重复: %d", level)
		}
		levelData, err := requireMap(levelNode, FileExp+".levels."+rawLevel)
		if err != nil {
			return err
		}
		maxNode, err := requireKey(levelData, "max", FileExp+".levels."+rawLevel)
		if err != nil {
			return err
		}
		maxExp, err := nonNegativeIntScalar(maxNode, FileExp+".levels."+rawLevel+".max")
		if err != nil {
			return err
		}
		e.byLevel[levelKey] = &LevelEntry{Level: level, MaxExp: maxExp}
		levelNumbers = append(levelNumbers, level)
	}

	sort.Ints(levelNumbers)
	expectedLevelCount := maxLevel - minLevel + 1
	if len(levelNumbers) != expectedLevelCount {
		return configError("经验等级数量必须完整覆盖协议范围: expected:%d actual:%d", expectedLevelCount, len(levelNumbers))
	}
	if levelNumbers[0] != minLevel {
		return configError("经验等级必须从协议最小等级开始: expected:%d actual:%d", minLevel, levelNumbers[0])
	}
	if levelNumbers[len(levelNumbers)-1] != maxLevel {
		return configError("经验等级必须覆盖到协议最大等级: expected:%d actual:%d", maxLevel, levelNumbers[len(levelNumbers)-1])
	}

	expectedLevel := minLevel
	var previous *LevelEntry
	for _, level := range levelNumbers {
		if level != expectedLevel {
			return configError("经验等级必须连续: expected:%d actual:%d", expectedLevel, level)
		}
		entry := e.byLevel[uint32(level)]
		if previous == nil {
			entry.MinExp = 0
		} else {
			if entry.MaxExp <= previous.MaxExp {
				return configError("经验等级 max 必须严格递增: level:%d max:%d previous_max:%d", level, entry.MaxExp, previous.MaxExp)
			}
			entry.MinExp = previous.MaxExp + 1
		}
		if entry.MinExp > entry.MaxExp {
			return configError("经验等级区间不能反向: level:%d min:%d max:%d", level, entry.MinExp, entry.MaxExp)
		}
		e.levels = append(e.levels, entry)
		previous = entry
		expectedLevel++
	}
	e.max = maxLevel
	return nil
}

func (e *ExpConfig) check() error {
	return nil
}

func (e *ExpConfig) assemble() error {
	return nil
}
