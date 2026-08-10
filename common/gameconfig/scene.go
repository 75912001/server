package gameconfig

import (
	"math"
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"

	pb "server/proto/pb"
)

type SceneConfig struct {
	*xmap.MapMgr[uint32, *SceneEntry]
}

type SceneEntry struct {
	// ID 来自 scenes[].id, 必须处于协议场景资源ID段内, 并且在 scene.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 scenes[].name, 表示场景名称, 主要用于配置识别, 日志和排障.
	Name *string `yaml:"name"`
	// EnemyCountMax 来自 scenes[].enemyCountMax, 保留场景遇敌区域元数据, 范围为1至10.
	// 当前普通敌组实际出怪数量由enemy.group.yaml的countRange决定.
	EnemyCountMax *uint32 `yaml:"enemyCountMax"`
	// EnemyGroups 来自 scenes[].enemyGroups, 保存当前场景可遇敌敌人组和权重.
	EnemyGroups []SceneEnemyGroupEntry `yaml:"enemyGroups"`
}

type SceneEnemyGroupEntry struct {
	// ID 来自 enemyGroups[].id, 必须引用 enemy.group.yaml 中存在的敌人组ID.
	ID *uint32 `yaml:"id"`
	// Weight 来自 enemyGroups[].weight, 映射8.5遇敌区域的敌组权重. 0不占随机区间且不会被抽中, 场景总权重必须大于0.
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
		if scene.EnemyCountMax == nil {
			return errors.Errorf("场景缺少 enemyCountMax: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if *scene.EnemyCountMax < uint32(pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Min) ||
			*scene.EnemyCountMax > uint32(pb.CombatCampPosition_CombatCampPosition_Count) {
			return errors.Errorf("场景 enemyCountMax 超出范围: scene:%d value:%d expected:[%d,%d] %v",
				*scene.ID, *scene.EnemyCountMax,
				pb.CombatEnemyGroupEnemyCountRange_CombatEnemyGroupEnemyCountRange_Min,
				pb.CombatCampPosition_CombatCampPosition_Count, xruntime.Location())
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
			if weight > uint32(math.MaxInt32) {
				return errors.Errorf("场景 enemyGroups[].weight 超出C int范围: scene:%d group:%d weight:%d %v",
					*scene.ID, groupID, weight, xruntime.Location())
			}
		}
		totalWeight := uint64(0)
		for i := range scene.EnemyGroups {
			totalWeight += uint64(*scene.EnemyGroups[i].Weight)
		}
		if totalWeight == 0 {
			return errors.Errorf("场景 enemyGroups 总权重必须大于0: scene:%d %v", *scene.ID, xruntime.Location())
		}
		if totalWeight > uint64(math.MaxInt32) {
			return errors.Errorf("场景 enemyGroups 总权重超出C int范围: scene:%d total:%d %v",
				*scene.ID, totalWeight, xruntime.Location())
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
