package gameconfig

func newCharacterConfig() *CharacterConfig {
	return &CharacterConfig{
		byID: map[int]*CharacterEntry{},
	}
}

func (c *CharacterConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FileCharacter)
	if err != nil {
		return err
	}
	characterNode, err := requireKey(root, "character", FileCharacter)
	if err != nil {
		return err
	}
	characters, err := requireSeq(characterNode, FileCharacter+".character")
	if err != nil {
		return err
	}
	if len(characters) == 0 {
		return configError("角色配置中没有解析到 character 数据: %s", FileCharacter)
	}

	for i, characterNode := range characters {
		path := configErrorPath(FileCharacter, "character", i)
		characterData, err := requireMap(characterNode, path)
		if err != nil {
			return err
		}
		entry, err := c.parseEntry(characterData, path)
		if err != nil {
			return err
		}
		if _, ok := c.byID[entry.ID]; ok {
			return configError("角色ID重复: %d", entry.ID)
		}
		c.byID[entry.ID] = entry
		c.ids = append(c.ids, entry.ID)
	}
	return nil
}

func (c *CharacterConfig) parseEntry(data yamlMap, path string) (*CharacterEntry, error) {
	idNode, err := requireKey(data, "id", path)
	if err != nil {
		return nil, err
	}
	id, err := intScalar(idNode, path+".id")
	if err != nil {
		return nil, err
	}
	if !isCharacterID(id) {
		return nil, configError("角色ID超出范围: %d", id)
	}
	isRole := false
	if isRoleNode, ok := data["isRole"]; ok {
		isRole, err = boolScalar(isRoleNode, path+".isRole")
		if err != nil {
			return nil, err
		}
	}
	return &CharacterEntry{
		ID:     id,
		IsRole: isRole,
	}, nil
}

func (c *CharacterConfig) check() error {
	return nil
}

func (c *CharacterConfig) assemble() error {
	return nil
}
