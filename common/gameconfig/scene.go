package gameconfig

var sceneKeys = stringSet("id", "name", "enemyGroups")
var sceneEnemyGroupKeys = stringSet("id", "weight")

func newSceneConfig() *SceneConfig {
	return &SceneConfig{
		byID: map[int]*SceneEntry{},
	}
}

func (s *SceneConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FileScene)
	if err != nil {
		return err
	}
	scenesNode, err := requireKey(root, "scenes", FileScene)
	if err != nil {
		return err
	}
	scenes, err := requireSeq(scenesNode, FileScene+".scenes")
	if err != nil {
		return err
	}
	if len(scenes) == 0 {
		return configError("场景配置中没有解析到 scenes 数据: %s", FileScene)
	}

	for i, sceneNode := range scenes {
		path := configErrorPath(FileScene, "scenes", i)
		sceneData, err := requireMap(sceneNode, path)
		if err != nil {
			return err
		}
		if err := assertKnownKeys(sceneData, sceneKeys, path); err != nil {
			return err
		}
		scene, err := s.parseScene(sceneData, path)
		if err != nil {
			return err
		}
		if _, ok := s.byID[scene.ID]; ok {
			return configError("场景ID重复: %d", scene.ID)
		}
		s.byID[scene.ID] = scene
		s.ids = append(s.ids, scene.ID)
	}
	return nil
}

func (s *SceneConfig) parseScene(data yamlMap, path string) (*SceneEntry, error) {
	idNode, err := requireKey(data, "id", path)
	if err != nil {
		return nil, err
	}
	id, err := intScalar(idNode, path+".id")
	if err != nil {
		return nil, err
	}
	if !isSceneID(id) {
		return nil, configError("场景ID超出协议范围: %d", id)
	}
	nameNode, err := requireKey(data, "name", path)
	if err != nil {
		return nil, err
	}
	name, err := stringScalar(nameNode, path+".name")
	if err != nil {
		return nil, err
	}
	enemyGroups, err := parseSceneEnemyGroups(data, id, path)
	if err != nil {
		return nil, err
	}
	return &SceneEntry{
		ID:          id,
		Name:        name,
		EnemyGroups: enemyGroups,
	}, nil
}

func parseSceneEnemyGroups(data yamlMap, sceneID int, path string) ([]SceneEnemyGroupEntry, error) {
	groupsNode, err := requireKey(data, "enemyGroups", path)
	if err != nil {
		return nil, err
	}
	groupNodes, err := requireSeq(groupsNode, path+".enemyGroups")
	if err != nil {
		return nil, err
	}
	if len(groupNodes) == 0 {
		return nil, configError("场景 enemyGroups 不能为空: scene:%d", sceneID)
	}

	out := make([]SceneEnemyGroupEntry, 0, len(groupNodes))
	for i, groupNode := range groupNodes {
		groupPath := path + ".enemyGroups[" + itoa(i) + "]"
		groupData, err := requireMap(groupNode, groupPath)
		if err != nil {
			return nil, err
		}
		if err := assertKnownKeys(groupData, sceneEnemyGroupKeys, groupPath); err != nil {
			return nil, err
		}
		idNode, err := requireKey(groupData, "id", groupPath)
		if err != nil {
			return nil, err
		}
		groupID, err := intScalar(idNode, groupPath+".id")
		if err != nil {
			return nil, err
		}
		if groupID <= 0 {
			return nil, configError("场景 enemyGroups[].id 非法: scene:%d group:%d", sceneID, groupID)
		}
		weightNode, err := requireKey(groupData, "weight", groupPath)
		if err != nil {
			return nil, err
		}
		weight, err := intScalar(weightNode, groupPath+".weight")
		if err != nil {
			return nil, err
		}
		if weight <= 0 {
			return nil, configError("场景 enemyGroups[].weight 必须大于0: scene:%d group:%d weight:%d", sceneID, groupID, weight)
		}
		out = append(out, SceneEnemyGroupEntry{ID: groupID, Weight: weight})
	}
	return out, nil
}

func (s *SceneConfig) check(enemyConfig *EnemyGroupConfig) error {
	if enemyConfig == nil {
		return nil
	}
	for _, sceneID := range s.ids {
		scene := s.byID[sceneID]
		for _, group := range scene.EnemyGroups {
			if enemyConfig.GetByID(group.ID) == nil {
				return configError("场景引用了未定义敌人组: scene:%d enemyGroup:%d", scene.ID, group.ID)
			}
		}
	}
	return nil
}

func (s *SceneConfig) assemble() error {
	return nil
}
