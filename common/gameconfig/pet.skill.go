package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type PetSkillConfig struct {
	*xmap.MapMgr[uint32, *PetSkillEntry]
}

type PetSkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议宠物技能资源ID段内, 并且在 pet.skill.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 skill[].name, 保留技能显示名称文本, server 不解析客户端资源.
	Name *string `yaml:"name"`
	// Description 来自 skill[].description, 保留技能说明文本, 可用于查询, 日志或后续业务展示下发.
	Description *string `yaml:"description"`
}

func newPetSkillConfig() *PetSkillConfig {
	return &PetSkillConfig{
		MapMgr: xmap.NewMapMgr[uint32, *PetSkillEntry](),
	}
}

func (p *PetSkillConfig) load(dir string) error {
	var root struct {
		Skill []*PetSkillEntry `yaml:"skill"`
	}
	if err := loadYAMLFile(dir, FilePetSkill, &root); err != nil {
		return err
	}
	return p.configure(root.Skill)
}

func (p *PetSkillConfig) configure(entries []*PetSkillEntry) error {
	for _, skill := range entries {
		if skill.ID == nil {
			return errors.Errorf("宠物技能缺少 id %v", xruntime.Location())
		}
		if !isPetSkillID(*skill.ID) {
			return errors.Errorf("宠物技能ID超出范围: %d %v", *skill.ID, xruntime.Location())
		}
		if skill.Name == nil {
			return errors.Errorf("宠物技能缺少 name: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Name) == "" {
			return errors.Errorf("宠物技能 name 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if skill.Description == nil {
			return errors.Errorf("宠物技能缺少 description: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Description) == "" {
			return errors.Errorf("宠物技能 description 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if !p.AddIfNotExist(*skill.ID, skill) {
			return errors.Errorf("宠物技能ID重复: %d %v", *skill.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *PetSkillConfig) check() error {
	return nil
}

func (p *PetSkillConfig) assemble() error {
	return nil
}
