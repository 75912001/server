package gameconfig

import (
	"strings"

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
	// Name 来自 character[].name, 供角色外观敌人的默认显示名称使用.
	Name *string `yaml:"name"`
	// IsRole 来自 character[].isRole, 标记该角色资源是否可作为玩家角色; 缺省时按 false 处理.
	IsRole *bool `yaml:"isRole"`
	// Mounts 是角色允许骑乘的宠物集合. 字段必须显式存在, 空数组表示该角色没有骑乘能力.
	Mounts []*CharacterMountEntry `yaml:"mounts"`
	// mountPetIDs 是完成表内校验后建立的查询索引, 骑乘请求不得绕过该权限集合.
	mountPetIDs map[uint32]struct{}
}

type CharacterMountEntry struct {
	// PetID 来自 mounts[].petId, 必须引用 pet.yaml 中存在的宠物且在同一角色内唯一.
	PetID *uint32 `yaml:"petId"`
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
		if character == nil {
			return errors.Errorf("角色配置条目不能为空 %v", xruntime.Location())
		}
		if character.ID == nil {
			return errors.Errorf("配置缺少必填字段: id %v", xruntime.Location())
		}
		if !isCharacterID(*character.ID) {
			return errors.Errorf("角色ID超出范围: %d %v", *character.ID, xruntime.Location())
		}
		if character.Name == nil || strings.TrimSpace(*character.Name) == "" {
			return errors.Errorf("角色配置缺少 name: character:%d %v", *character.ID, xruntime.Location())
		}
		defaultBool(&character.IsRole, false)
		if character.Mounts == nil {
			return errors.Errorf("角色配置缺少必填字段: mounts character:%d %v", *character.ID, xruntime.Location())
		}
		character.mountPetIDs = make(map[uint32]struct{}, len(character.Mounts))
		for index, mount := range character.Mounts {
			if mount == nil || mount.PetID == nil {
				return errors.Errorf("角色骑乘配置缺少petId: character:%d index:%d %v", *character.ID, index, xruntime.Location())
			}
			if !isPetID(*mount.PetID) {
				return errors.Errorf("角色骑乘宠物ID超出范围: character:%d pet:%d %v", *character.ID, *mount.PetID, xruntime.Location())
			}
			if _, exists := character.mountPetIDs[*mount.PetID]; exists {
				return errors.Errorf("角色骑乘宠物ID重复: character:%d pet:%d %v", *character.ID, *mount.PetID, xruntime.Location())
			}
			character.mountPetIDs[*mount.PetID] = struct{}{}
		}
		if !p.AddIfNotExist(*character.ID, character) {
			return errors.Errorf("角色ID重复: %d %v", *character.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *CharacterConfig) check() error {
	if GGameConfig == nil || GGameConfig.Pet == nil {
		return errors.Errorf("角色配置缺少宠物配置依赖 %v", xruntime.Location())
	}
	var checkErr error
	p.Foreach(func(_ uint32, character *CharacterEntry) bool {
		for petID := range character.mountPetIDs {
			if GGameConfig.Pet.Get(petID) == nil {
				checkErr = errors.Errorf("角色骑乘配置引用不存在的宠物: character:%d pet:%d %v", *character.ID, petID, xruntime.Location())
				return false
			}
		}
		return true
	})
	return checkErr
}

func (p *CharacterConfig) assemble() error {
	return nil
}

// CanMount 返回该角色是否显式配置了指定宠物的骑乘能力.
func (p *CharacterEntry) CanMount(petID uint32) bool {
	if p == nil {
		return false
	}
	_, ok := p.mountPetIDs[petID]
	return ok
}
