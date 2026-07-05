package gameconfig

import pb "server/proto/pb"

func newEnemyGroupConfig() *EnemyGroupConfig {
	return &EnemyGroupConfig{
		byID: map[uint32]*EnemyGroupEntry{},
	}
}

func (p *EnemyGroupConfig) load(dir string) error {
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
		group, err := p.parseGroup(groupData, path)
		if err != nil {
			return err
		}
		if _, ok := p.byID[group.ID]; ok {
			return configError("敌人组ID重复: %d", group.ID)
		}
		p.byID[group.ID] = group
		p.ids = append(p.ids, group.ID)
	}
	return nil
}

func (p *EnemyGroupConfig) parseGroup(data yamlMap, path string) (*EnemyGroupEntry, error) {
	idNode, err := requireKey(data, "id", path)
	if err != nil {
		return nil, err
	}
	groupID, err := uint32Scalar(idNode, path+".id")
	if err != nil {
		return nil, err
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
		ID:              groupID,
		Name:            name,
		IsBoss:          isBoss,
		CountRange:      IntRange{Min: 1, Max: 1},
		LevelRange:      IntRange{Min: 1, Max: 1},
		RoleLevelOffset: IntRange{Min: 0, Max: 0},
		Captured:        true,
	}
	if group.IsBoss {
		if err := assertAbsent(data, "countRange", "Boss 敌人组 countRange 无效, 不应配置: group:%d", groupID); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "levelRange", "Boss 敌人组 levelRange 无效, 不应配置: group:%d", groupID); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "roleLevelOffset", "Boss 敌人组 roleLevelOffset 无效, 不应配置: group:%d", groupID); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "captured", "Boss 敌人组 captured 无效, 不应配置: group:%d", groupID); err != nil {
			return nil, err
		}
		if err := assertAbsent(data, "babyRate", "Boss 敌人组 babyRate 无效, 不应配置: group:%d", groupID); err != nil {
			return nil, err
		}
		group.Captured = false
		group.BabyRate = 0
	} else {
		if _, ok := data["countRange"]; !ok {
			return nil, configError("普通敌人组缺少 countRange: group:%d", groupID)
		}
		if _, ok := data["levelRange"]; !ok {
			if _, hasRoleLevelOffset := data["roleLevelOffset"]; !hasRoleLevelOffset {
				return nil, configError("普通敌人组缺少 levelRange 或 roleLevelOffset: group:%d", groupID)
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
			group.BabyRate, err = uint32Scalar(babyRateNode, path+".babyRate")
			if err != nil {
				return nil, err
			}
			if group.BabyRate < uint32(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Min) || group.BabyRate > uint32(pb.CombatEnemyGroupBabyRate_CombatEnemyGroupBabyRate_Max) {
				return nil, configError("敌人组 babyRate 超出范围: group:%d value:%d", groupID, group.BabyRate)
			}
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
		idNode, err := requireKey(enemyData, "id", enemyPath)
		if err != nil {
			return nil, err
		}
		enemyID, err := uint32Scalar(idNode, enemyPath+".id")
		if err != nil {
			return nil, err
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
			enemy.Level, err := uint32Scalar(levelNode, enemyPath+".level")
			if err != nil {
				return nil, err
			}
			if enemy.Level < uint32(pb.LevelRange_LevelRange_Min) || uint32(pb.LevelRange_LevelRange_Max) < enemy.Level {
				return nil, configError("敌人组 enemy level 超出范围: group:%d enemy:%d level:%d", group.ID, enemy.ID, enemy.Level)
			}
			enemy.Weight = 0
		} else {
			if weightNode, ok := enemyData["weight"]; ok {
				enemy.Weight, err = uint32Scalar(weightNode, enemyPath+".weight")
				if err != nil {
					return nil, err
				}
			}
			if levelNode, ok := enemyData["level"]; ok {
				enemy.Level, err = uint32Scalar(levelNode, enemyPath+".level")
				if err != nil {
					return nil, err
				}
				if enemy.Level < uint32(pb.LevelRange_LevelRange_Min) || uint32(pb.LevelRange_LevelRange_Max) < enemy.Level {
					return nil, configError("敌人组 enemy level 超出范围: group:%d enemy:%d level:%d", group.ID, enemy.ID, enemy.Level)
				}
			}
		}
		out = append(out, enemy)
	}
	return out, nil
}

func (p *EnemyGroupConfig) check(petConfig *PetConfig) error {
	if petConfig == nil {
		return nil
	}
	for _, groupID := range p.ids {
		group := p.byID[groupID]
		for _, enemy := range group.Enemies {
			if !petConfig.HasID(enemy.ID) {
				return configError("敌人组引用了未定义宠物: group:%d pet:%d", group.ID, enemy.ID)
			}
		}
	}
	return nil
}

func (p *EnemyGroupConfig) assemble() error {
	return nil
}
