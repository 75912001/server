package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type SkillConfig struct {
	*xmap.MapMgr[uint32, *SkillEntry]
}

type SkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议技能共用资源ID段内, 并且在 skill.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 skill[].name, 保留技能显示名称文本, server 不解析客户端资源.
	Name *string `yaml:"name"`
	// Description 来自 skill[].description, 保留技能说明文本, 可用于查询、日志或后续业务展示下发.
	Description *string `yaml:"description"`
	// ContinuationAttack 非 nil 表示连续攻击, 段数由服务端配置决定.
	ContinuationAttack *SkillContinuationAttackEntry `yaml:"continuationAttack"`
}

type SkillContinuationAttackEntry struct {
	SegmentCount *uint32 `yaml:"segmentCount"`
}

func (p *SkillContinuationAttackEntry) check() error {
	if p.SegmentCount == nil {
		return errors.Errorf("连续攻击缺少 continuationAttack.segmentCount %v", xruntime.Location())
	}
	if *p.SegmentCount < 1 || *p.SegmentCount > 10 {
		return errors.Errorf("连续攻击 continuationAttack.segmentCount 超出1至10范围: value:%d %v", *p.SegmentCount, xruntime.Location())
	}
	return nil
}

func newSkillConfig() *SkillConfig {
	return &SkillConfig{
		MapMgr: xmap.NewMapMgr[uint32, *SkillEntry](),
	}
}

func (p *SkillConfig) load(dir string) error {
	var root struct {
		Skill []*SkillEntry `yaml:"skill"`
	}
	if err := loadYAMLFile(dir, FileSkill, &root); err != nil {
		return err
	}
	return p.configure(root.Skill)
}

func (p *SkillConfig) configure(entries []*SkillEntry) error {
	for _, skill := range entries {
		if skill.ID == nil {
			return errors.Errorf("技能缺少 id %v", xruntime.Location())
		}
		if !isSkillID(*skill.ID) {
			return errors.Errorf("技能ID超出范围: %d %v", *skill.ID, xruntime.Location())
		}
		if skill.Name == nil {
			return errors.Errorf("技能缺少 name: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Name) == "" {
			return errors.Errorf("技能 name 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if skill.Description == nil {
			return errors.Errorf("技能缺少 description: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Description) == "" {
			return errors.Errorf("技能 description 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if skill.ContinuationAttack != nil {
			if err := skill.ContinuationAttack.check(); err != nil {
				return errors.Wrapf(err, "技能参数错误: ID:%d", *skill.ID)
			}
		}
		if !p.AddIfNotExist(*skill.ID, skill) {
			return errors.Errorf("技能ID重复: %d %v", *skill.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *SkillConfig) check() error {
	return nil
}

func (p *SkillConfig) assemble() error {
	return nil
}
