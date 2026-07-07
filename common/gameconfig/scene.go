package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type SceneConfig struct {
	*xmap.MapMgr[uint32, *SceneEntry]
}

type SceneEntry struct {
	// ID 来自 scenes[].id, 必须处于协议场景资源ID段内, 并且在 scene.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 scenes[].name, 表示场景名称, 主要用于配置识别, 日志和排障.
	Name *string `yaml:"name"`
	// EnemyGroups 来自 scenes[].enemyGroups, 保存当前场景可遇敌敌人组和权重.
	EnemyGroups []SceneEnemyGroupEntry `yaml:"enemyGroups"`
}

type SceneEnemyGroupEntry struct {
	// ID 来自 enemyGroups[].id, 必须引用 enemy.group.yaml 中存在的敌人组ID.
	ID *uint32 `yaml:"id"`
	// Weight 来自 enemyGroups[].weight, 表示当前场景选择该敌人组的权重, 必须大于0.
	Weight *uint32 `yaml:"weight"`
}

func newSceneConfig() *SceneConfig {
	return &SceneConfig{
		MapMgr: xmap.NewMapMgr[uint32, *SceneEntry](),
	}
}

func (p *SceneConfig) load(dir string) error {
	var root struct {
		Scenes []*SceneEntry `yaml:"scenes"`
	}
	if err := loadYAMLFile(dir, FileScene, &root); err != nil {
		return err
	}
	return p.configure(root.Scenes)
}

func (p *SceneConfig) configure(entries []*SceneEntry) error {
	for _, scene := range entries {
		if scene.ID == nil {
			return errors.Errorf("场景缺少 id %v", xruntime.Location())
		}
		if !isSceneID(*scene.ID) {
			return errors.Errorf("场景ID超出协议范围: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if scene.Name == nil {
			return errors.Errorf("场景缺少 name: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if strings.TrimSpace(*scene.Name) == "" {
			return errors.Errorf("场景 name 不能为空: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if scene.EnemyGroups == nil {
			return errors.Errorf("场景缺少 enemyGroups: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if len(scene.EnemyGroups) == 0 {
			return errors.Errorf("场景 enemyGroups 不能为空: scene:%d %v", *scene.ID, xruntime.Location())
		}
		for i := range scene.EnemyGroups {
			if scene.EnemyGroups[i].ID == nil {
				return errors.Errorf("场景 enemyGroups 缺少 id: scene:%d index:%d %v", *scene.ID, i, xruntime.Location())
			}
			groupID := *scene.EnemyGroups[i].ID
			if scene.EnemyGroups[i].Weight == nil {
				return errors.Errorf("场景 enemyGroups 缺少 weight: scene:%d group:%d %v", *scene.ID, groupID, xruntime.Location())
			}
			weight := *scene.EnemyGroups[i].Weight
			if weight == 0 {
				return errors.Errorf("场景 enemyGroups[].weight 必须大于0: scene:%d group:%d weight:%d %v", *scene.ID, groupID, weight, xruntime.Location())
			}
		}
		if !p.AddIfNotExist(*scene.ID, scene) {
			return errors.Errorf("场景ID重复: scene:%d %v", *scene.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *SceneConfig) check() error {
	var err error
	p.Foreach(func(sceneID uint32, scene *SceneEntry) bool {
		for _, group := range scene.EnemyGroups {
			if GGameConfig.Enemy.Get(*group.ID) == nil {
				err = errors.Errorf("场景引用了未定义敌人组: scene:%d enemyGroup:%d %v", sceneID, *group.ID, xruntime.Location())
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *SceneConfig) assemble() error {
	return nil
}
