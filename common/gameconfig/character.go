package gameconfig

type CharacterConfig struct {
	Character []*CharacterEntry `yaml:"character"`
	byID      map[uint32]*CharacterEntry
}

type CharacterEntry struct {
	// ID 来自 character[].id, 必须处于协议角色资源ID段内, 并且在 character.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// IsRole 来自 character[].isRole, 标记该角色资源是否可作为玩家角色; 缺省时按 false 处理.
	IsRole *bool `yaml:"isRole"`
}

func (p *CharacterConfig) GetByID(id uint32) *CharacterEntry {
	return p.byID[id]
}

func newCharacterConfig() *CharacterConfig {
	return &CharacterConfig{
		byID: map[uint32]*CharacterEntry{},
	}
}

func (p *CharacterConfig) load(dir string) error {
	if err := loadYAMLFile(dir, FileCharacter, p); err != nil {
		return err
	}
	return p.configure()
}

func (p *CharacterConfig) configure() error {
	p.byID = map[uint32]*CharacterEntry{}
	for i, character := range p.Character {
		path := configErrorPath(FileCharacter, "character", i)
		if character == nil {
			continue
		}
		if character.ID == nil {
			return configError("配置缺少必填字段: %s.id", path)
		}
		if !isCharacterID(*character.ID) {
			return configError("角色ID超出范围: %d", *character.ID)
		}
		defaultBool(&character.IsRole, false)
		if _, ok := p.byID[*character.ID]; ok {
			return configError("角色ID重复: %d", *character.ID)
		}
		p.byID[*character.ID] = character
	}
	return nil
}

func (p *CharacterConfig) check() error {
	return nil
}

func (p *CharacterConfig) assemble() error {
	return nil
}
