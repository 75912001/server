package gameconfig

import (
	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type CharacterConfig struct {
	*xmap.MapMgr[uint32, *CharacterEntry]
}

type CharacterEntry struct {
	// ID 来自 character[].id, 必须处于协议角色资源ID段内, 并且在 character.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// IsRole 来自 character[].isRole, 标记该角色资源是否可作为玩家角色; 缺省时按 false 处理.
	IsRole *bool `yaml:"isRole"`
}

func newCharacterConfig() *CharacterConfig {
	return &CharacterConfig{
		MapMgr: xmap.NewMapMgr[uint32, *CharacterEntry](),
	}
}

func (p *CharacterConfig) load(dir string) error {
	var root struct {
		Character []*CharacterEntry `yaml:"character"`
	}
	if err := loadYAMLFile(dir, FileCharacter, &root); err != nil {
		return err
	}
	return p.configure(root.Character)
}

func (p *CharacterConfig) configure(entries []*CharacterEntry) error {
	for _, character := range entries {
		if character.ID == nil {
			return errors.Errorf("配置缺少必填字段: id %v", xruntime.Location())
		}
		if !isCharacterID(*character.ID) {
			return errors.Errorf("角色ID超出范围: %d %v", *character.ID, xruntime.Location())
		}
		defaultBool(&character.IsRole, false)
		if !p.AddIfNotExist(*character.ID, character) {
			return errors.Errorf("角色ID重复: %d %v", *character.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *CharacterConfig) check() error {
	return nil
}

func (p *CharacterConfig) assemble() error {
	return nil
}
