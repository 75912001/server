package gameconfig

import pb "server/proto/pb"

var enemyGroupKeys = stringSet("id", "name", "isBoss", "countRange", "levelRange", "roleLevelOffset", "captured", "babyRate", "enemies")
var enemyKeys = stringSet("id", "weight", "level")

func newEnemyGroupConfig() *EnemyGroupConfig {
	return &EnemyGroupConfig{
		byID: map[int]*EnemyGroupEntry{},
	}
}

func (e *EnemyGroupConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FileEnemyGroup)
	if err != nil {
		return err
	}
	groupsNode, err := requireKey(root, "enemyGroups", FileEnemyGroup)
	if err != nil {
		return err
	}
	groups, err := requireSeq(groupsNode, FileEnemyGroup+".enemyGroups")
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return configError("敌人组配置中没有解析到 enemyGroups 数据: %s", FileEnemyGroup)
	}

	for i, groupNode := range groups {
		path := configErrorPath(FileEnemyGroup, "enemyGroups", i)
		groupData, err := requireMap(groupNode, path)
		if err != nil {
			return err
		}
		if err := assertKnownKeys(groupData, enemyGroupKeys, path); err != nil {
			return err
		}
		group, err := e.parseGroup(groupData, path)
		if err != nil {
			return err
		}
		if _, ok := e.byID[group.ID]; ok {
			return configError("敌人组ID重复: %d", group.ID)
		}
		e.byID[group.ID] = group
		e.ids = append(e.ids, group.ID)
	}
	return nil
}

func (e *EnemyGroupConfig) parseGroup(data yamlMap, path string) (*EnemyGroupEntry, error) {
	idNode, err := requireKey(data, "id", path)
	if err != nil {
		return nil, err
	}
	id, err := intScalar(idNode, path+".id")
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, configError("敌人组ID非法: %d", id)
	}
	nameNode, err := requireKey(data, "name", path)
	if err != nil {
		return nil, err
	}
	name, err := stringScalar(nameNode, path+".name")
	if err != nil {
		return nil, err
	}
	isBoss := false
	if isBossNode, ok := data["isBoss"]; ok {
		isBoss, err = boolScalar(isBossNode, path+".isBoss")
		if err != nil {
			return nil, err
		}
	}

	group := &EnemyGroupEntry{
		ID:              id,
		Name:            name,
		IsBoss:          isBoss,
		CountRange:      IntRange{Min: 1, Max: 1},
		LevelRange:      IntRange{Min: 1, Max: 1},
		RoleLevelOffset: IntRange{Min: 0, Max: 0},
		Captured:        true,
	}
	if group.IsBoss {
		if err := assertAbsent(data, "countRange", "Boss 敌人组 countRange 无效, 不应配置: group:%d", id); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "levelRange", "Boss 敌人组 levelRange 无效, 不应配置: group:%d", id); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "roleLevelOffset", "Boss 敌人组 roleLevelOffset 无效, 不应配置: group:%d", id); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "captured", "Boss 敌人组 captured 无效, 不应配置: group:%d", id); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "babyRate", "Boss 敌人组 babyRate 无效, 不应配置: group:%d", id); err != nil {
			return nil, err
		}
		group.Captured = false
		group.BabyRate = 0
	} else {
		if _, ok := data["countRange"]; !ok {
			return nil, configError("普通敌人组缺少 countRange: group:%d", id)
		}
		if _, hasLevelRange := data["levelRange"]; !hasLevelRange {
			if _, hasRoleLevelOffset := data["roleLevelOffset"]; !hasRoleLevelOffset {
				return nil, configError("普通敌人组缺少 levelRange 或 roleLevelOffset: group:%d", id)
			}
		}
		group.CountRange, err = intRange(data["countRange"], path+".countRange")
		if err != nil {
			return nil, err
		}
		if err = assertRangeBounds(group.CountRange, int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Min), int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Max), path+".countRange"); err != nil {
			return nil, err
		}
		if levelRangeNode, ok := data["levelRange"]; ok {
			group.LevelRange, err = intRange(levelRangeNode, path+".levelRange")
			if err != nil {
				return nil, err
			}
			if err = assertRangeBounds(group.LevelRange, int(pb.LevelRange_LevelRange_Min), int(pb.LevelRange_LevelRange_Max), path+".levelRange"); err != nil {
				return nil, err
			}
		}
		if roleLevelOffsetNode, ok := data["roleLevelOffset"]; ok {
			group.RoleLevelOffset, err = intRange(roleLevelOffsetNode, path+".roleLevelOffset")
			if err != nil {
				return nil, err
			}
		}
		if capturedNode, ok := data["captured"]; ok {
			group.Captured, err = boolScalar(capturedNode, path+".captured")
			if err != nil {
				return nil, err
			}
		}
		if babyRateNode, ok := data["babyRate"]; ok {
			group.BabyRate, err = intScalar(babyRateNode, path+".babyRate")
			if err != nil {
				return nil, err
			}
		}
		if group.BabyRate < int(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Min) || group.BabyRate > int(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Max) {
			return nil, configError("敌人组 babyRate 超出范围: group:%d value:%d", id, group.BabyRate)
		}
	}

	enemies, err := parseEnemies(data, group, path)
	if err != nil {
		return nil, err
	}
	group.Enemies = enemies
	return group, nil
}

func parseEnemies(data yamlMap, group *EnemyGroupEntry, path string) ([]EnemyEntry, error) {
	enemiesNode, err := requireKey(data, "enemies", path)
	if err != nil {
		return nil, err
	}
	enemyNodes, err := requireSeq(enemiesNode, path+".enemies")
	if err != nil {
		return nil, err
	}
	if len(enemyNodes) == 0 {
		return nil, configError("敌人组 enemies 不能为空: group:%d", group.ID)
	}
	if len(enemyNodes) > int(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Max) {
		return nil, configError("敌人组 enemies 超过最大站位数量: group:%d size:%d", group.ID, len(enemyNodes))
	}

	out := make([]EnemyEntry, 0, len(enemyNodes))
	for i, enemyNode := range enemyNodes {
		enemyPath := path + ".enemies[" + itoa(i) + "]"
		enemyData, err := requireMap(enemyNode, enemyPath)
		if err != nil {
			return nil, err
		}
		if err := assertKnownKeys(enemyData, enemyKeys, enemyPath); err != nil {
			return nil, err
		}
		idNode, err := requireKey(enemyData, "id", enemyPath)
		if err != nil {
			return nil, err
		}
		enemyID, err := intScalar(idNode, enemyPath+".id")
		if err != nil {
			return nil, err
		}
		if enemyID <= 0 {
			return nil, configError("敌人组 enemy id 非法: group:%d enemy:%d", group.ID, enemyID)
		}
		enemy := EnemyEntry{ID: enemyID}
		if group.IsBoss {
			if err := assertAbsent(enemyData, "weight", "Boss 敌人组 enemy.weight 无效, 不应配置: group:%d enemy:%d", group.ID, enemyID); err != nil {
				return nil, err
			}
			levelNode, err := requireKey(enemyData, "level", enemyPath)
			if err != nil {
				return nil, err
			}
			enemy.Level, err = intScalar(levelNode, enemyPath+".level")
			if err != nil {
				return nil, err
			}
			if err := assertEnemyLevel(group.ID, enemy.ID, enemy.Level); err != nil {
				return nil, err
			}
			enemy.Weight = 0
		} else {
			if weightNode, ok := enemyData["weight"]; ok {
				enemy.Weight, err = intScalar(weightNode, enemyPath+".weight")
				if err != nil {
					return nil, err
				}
			}
			if enemy.Weight < 0 {
				return nil, configError("敌人组 enemy weight 不能为负数: group:%d enemy:%d weight:%d", group.ID, enemy.ID, enemy.Weight)
			}
			if levelNode, ok := enemyData["level"]; ok {
				enemy.Level, err = intScalar(levelNode, enemyPath+".level")
				if err != nil {
					return nil, err
				}
				if err := assertEnemyLevel(group.ID, enemy.ID, enemy.Level); err != nil {
					return nil, err
				}
			}
		}
		out = append(out, enemy)
	}
	return out, nil
}

func assertEnemyLevel(groupID int, enemyID int, level int) error {
	if level < int(pb.LevelRange_LevelRange_Min) || level > int(pb.LevelRange_LevelRange_Max) {
		return configError("敌人组 enemy level 超出范围: group:%d enemy:%d level:%d", groupID, enemyID, level)
	}
	return nil
}

func (e *EnemyGroupConfig) check(petConfig *PetConfig) error {
	if petConfig == nil {
		return nil
	}
	for _, groupID := range e.ids {
		group := e.byID[groupID]
		for _, enemy := range group.Enemies {
			if !petConfig.HasID(enemy.ID) {
				return configError("敌人组引用了未定义宠物: group:%d pet:%d", group.ID, enemy.ID)
			}
		}
	}
	return nil
}

func (e *EnemyGroupConfig) assemble() error {
	return nil
}
