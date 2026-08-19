package gameconfig

import (
	"math"
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

const sceneConfigFormat = "sa-scene-v1"

type SceneConfig struct {
	*xmap.MapMgr[uint32, *SceneEntry]
}

type SceneEntry struct {
	ID        *uint32              `yaml:"id"`
	Name      *string              `yaml:"name"`
	Width     *uint32              `yaml:"width"`
	Height    *uint32              `yaml:"height"`
	Collision *SceneCollisionEntry `yaml:"collision"`
	Encounter *SceneEncounterEntry `yaml:"encounter"`
	Warps     []SceneWarpEntry     `yaml:"warps"`
}

type SceneCollisionEntry struct {
	BlockedRows [][]uint32 `yaml:"blockedRows"`
}

type SceneEncounterEntry struct {
	Default *SceneEncounterRuleEntry     `yaml:"default"`
	Regions []*SceneEncounterRegionEntry `yaml:"regions"`
}

type SceneEncounterRuleEntry struct {
	Enabled     *bool                  `yaml:"enabled"`
	EnemyGroups []SceneEnemyGroupEntry `yaml:"enemyGroups"`
}

type SceneEncounterRegionEntry struct {
	ID          *uint32                `yaml:"id"`
	Name        *string                `yaml:"name"`
	Enabled     *bool                  `yaml:"enabled"`
	Rows        [][]uint32             `yaml:"rows"`
	EnemyGroups []SceneEnemyGroupEntry `yaml:"enemyGroups"`
}

type SceneEnemyGroupEntry struct {
	ID     *uint32 `yaml:"id"`
	Weight *uint32 `yaml:"weight"`
}

type SceneWarpEntry struct {
	ID           *uint32                     `yaml:"id"`
	Name         *string                     `yaml:"name"`
	X            *uint32                     `yaml:"x"`
	Y            *uint32                     `yaml:"y"`
	Trigger      *string                     `yaml:"trigger"`
	Selection    *string                     `yaml:"selection"`
	Destinations []SceneWarpDestinationEntry `yaml:"destinations"`
}

type SceneWarpDestinationEntry struct {
	Condition *string               `yaml:"condition"`
	Target    *SceneWarpTargetEntry `yaml:"target"`
}

type SceneWarpTargetEntry struct {
	MapID *uint32 `yaml:"mapId"`
	X     *uint32 `yaml:"x"`
	Y     *uint32 `yaml:"y"`
}

func newSceneConfig() *SceneConfig {
	return &SceneConfig{
		MapMgr: xmap.NewMapMgr[uint32, *SceneEntry](),
	}
}

func (p *SceneConfig) load(dir string) error {
	var root struct {
		Format string        `yaml:"format"`
		Scenes []*SceneEntry `yaml:"scenes"`
	}
	if err := loadYAMLFile(dir, FileScene, &root); err != nil {
		return err
	}
	if root.Format != sceneConfigFormat {
		return errors.Errorf("scene.yaml格式无效: got:%q expected:%q %v", root.Format, sceneConfigFormat, xruntime.Location())
	}
	return p.configure(root.Scenes)
}

func (p *SceneConfig) configure(entries []*SceneEntry) error {
	for index, scene := range entries {
		if scene == nil {
			return errors.Errorf("场景条目为空: index:%d %v", index, xruntime.Location())
		}
		if scene.ID == nil {
			return errors.Errorf("场景缺少 id %v", xruntime.Location())
		}
		sceneID := *scene.ID
		if !isSceneID(sceneID) {
			return errors.Errorf("场景ID超出协议范围: scene:%d %v", sceneID, xruntime.Location())
		}
		if scene.Name == nil || strings.TrimSpace(*scene.Name) == "" {
			return errors.Errorf("场景 name 不能为空: scene:%d %v", sceneID, xruntime.Location())
		}
		if scene.Width == nil || scene.Height == nil || *scene.Width == 0 || *scene.Height == 0 {
			return errors.Errorf("场景尺寸无效: scene:%d %v", sceneID, xruntime.Location())
		}
		if scene.Collision == nil || scene.Collision.BlockedRows == nil {
			return errors.Errorf("场景缺少 collision.blockedRows: scene:%d %v", sceneID, xruntime.Location())
		}
		blockedCells := make(map[uint64]uint32)
		if err := validateSceneRows(
			"collision.blockedRows", scene.Collision.BlockedRows, *scene.Width, *scene.Height, blockedCells, 1,
		); err != nil {
			return errors.Wrapf(err, "scene:%d", sceneID)
		}
		if scene.Encounter == nil || scene.Encounter.Default == nil || scene.Encounter.Regions == nil {
			return errors.Errorf("场景缺少 encounter.default 或 encounter.regions: scene:%d %v", sceneID, xruntime.Location())
		}
		if err := validateSceneEncounterRule(sceneID, "default", scene.Encounter.Default.Enabled, scene.Encounter.Default.EnemyGroups); err != nil {
			return err
		}
		regionIDs := make(map[uint32]struct{}, len(scene.Encounter.Regions))
		regionCells := make(map[uint64]uint32)
		for regionIndex, region := range scene.Encounter.Regions {
			if region == nil || region.ID == nil || *region.ID == 0 {
				return errors.Errorf("场景遇敌区域ID无效: scene:%d index:%d %v", sceneID, regionIndex, xruntime.Location())
			}
			regionID := *region.ID
			if _, exists := regionIDs[regionID]; exists {
				return errors.Errorf("场景遇敌区域ID重复: scene:%d region:%d %v", sceneID, regionID, xruntime.Location())
			}
			regionIDs[regionID] = struct{}{}
			if region.Name == nil || strings.TrimSpace(*region.Name) == "" {
				return errors.Errorf("场景遇敌区域名称为空: scene:%d region:%d %v", sceneID, regionID, xruntime.Location())
			}
			if region.Rows == nil {
				return errors.Errorf("场景遇敌区域缺少 rows: scene:%d region:%d %v", sceneID, regionID, xruntime.Location())
			}
			if err := validateSceneEncounterRule(
				sceneID, "region", region.Enabled, region.EnemyGroups,
			); err != nil {
				return errors.Wrapf(err, "region:%d", regionID)
			}
			beforeCount := len(regionCells)
			if err := validateSceneRows(
				"encounter.regions.rows", region.Rows, *scene.Width, *scene.Height, regionCells, regionID,
			); err != nil {
				return errors.Wrapf(err, "scene:%d region:%d", sceneID, regionID)
			}
			if len(regionCells) > beforeCount {
				for _, row := range region.Rows {
					for x := row[1]; x <= row[2]; x++ {
						key := sceneCellKey(x, row[0])
						if _, blocked := blockedCells[key]; blocked {
							return errors.Errorf(
								"场景遇敌区域包含阻挡格: scene:%d region:%d x:%d y:%d %v",
								sceneID, regionID, x, row[0], xruntime.Location(),
							)
						}
					}
				}
			}
		}
		if scene.Warps == nil {
			return errors.Errorf("场景缺少 warps: scene:%d %v", sceneID, xruntime.Location())
		}
		if err := validateSceneWarps(scene); err != nil {
			return err
		}
		if !p.AddIfNotExist(sceneID, scene) {
			return errors.Errorf("场景ID重复: scene:%d %v", sceneID, xruntime.Location())
		}
	}
	return nil
}

func validateSceneRows(
	label string,
	rows [][]uint32,
	width uint32,
	height uint32,
	occupied map[uint64]uint32,
	owner uint32,
) error {
	for index, row := range rows {
		if len(row) != 3 {
			return errors.Errorf("%s行格式错误: index:%d %v", label, index, xruntime.Location())
		}
		y, startX, endX := row[0], row[1], row[2]
		if y >= height || startX > endX || endX >= width {
			return errors.Errorf(
				"%s坐标越界: index:%d row:%v width:%d height:%d %v",
				label, index, row, width, height, xruntime.Location(),
			)
		}
		for x := startX; x <= endX; x++ {
			key := sceneCellKey(x, y)
			if previousOwner, exists := occupied[key]; exists {
				return errors.Errorf(
					"%s格子重叠: x:%d y:%d owner:%d previous:%d %v",
					label, x, y, owner, previousOwner, xruntime.Location(),
				)
			}
			occupied[key] = owner
		}
	}
	return nil
}

func validateSceneEncounterRule(
	sceneID uint32,
	label string,
	enabled *bool,
	groups []SceneEnemyGroupEntry,
) error {
	if enabled == nil {
		return errors.Errorf("场景遇敌规则缺少 enabled: scene:%d rule:%s %v", sceneID, label, xruntime.Location())
	}
	if groups == nil {
		return errors.Errorf("场景遇敌规则缺少 enemyGroups: scene:%d rule:%s %v", sceneID, label, xruntime.Location())
	}
	if *enabled && len(groups) == 0 {
		return errors.Errorf("启用的场景遇敌规则 enemyGroups 不能为空: scene:%d rule:%s %v", sceneID, label, xruntime.Location())
	}
	groupIDs := make(map[uint32]struct{}, len(groups))
	totalWeight := uint64(0)
	for index := range groups {
		group := &groups[index]
		if group.ID == nil || *group.ID == 0 {
			return errors.Errorf("场景遇敌规则敌人组ID无效: scene:%d rule:%s index:%d %v", sceneID, label, index, xruntime.Location())
		}
		if _, exists := groupIDs[*group.ID]; exists {
			return errors.Errorf("场景遇敌规则敌人组ID重复: scene:%d rule:%s group:%d %v", sceneID, label, *group.ID, xruntime.Location())
		}
		groupIDs[*group.ID] = struct{}{}
		if group.Weight == nil || *group.Weight > uint32(math.MaxInt32) {
			return errors.Errorf("场景遇敌规则权重无效: scene:%d rule:%s group:%d %v", sceneID, label, *group.ID, xruntime.Location())
		}
		totalWeight += uint64(*group.Weight)
	}
	if len(groups) > 0 && (totalWeight == 0 || totalWeight > uint64(math.MaxInt32)) {
		return errors.Errorf(
			"场景遇敌规则总权重无效: scene:%d rule:%s total:%d %v",
			sceneID, label, totalWeight, xruntime.Location(),
		)
	}
	return nil
}

func validateSceneWarps(scene *SceneEntry) error {
	warpIDs := make(map[uint32]struct{}, len(scene.Warps))
	for index, warp := range scene.Warps {
		if warp.ID == nil || *warp.ID == 0 {
			return errors.Errorf("场景传送点ID无效: scene:%d index:%d %v", *scene.ID, index, xruntime.Location())
		}
		if _, exists := warpIDs[*warp.ID]; exists {
			return errors.Errorf("场景传送点ID重复: scene:%d warp:%d %v", *scene.ID, *warp.ID, xruntime.Location())
		}
		warpIDs[*warp.ID] = struct{}{}
		if warp.Name == nil || strings.TrimSpace(*warp.Name) == "" ||
			warp.Trigger == nil || strings.TrimSpace(*warp.Trigger) == "" ||
			warp.Selection == nil || strings.TrimSpace(*warp.Selection) == "" {
			return errors.Errorf("场景传送点名称或策略为空: scene:%d warp:%d %v", *scene.ID, *warp.ID, xruntime.Location())
		}
		if warp.X == nil || warp.Y == nil || *warp.X >= *scene.Width || *warp.Y >= *scene.Height {
			return errors.Errorf("场景传送点坐标越界: scene:%d warp:%d %v", *scene.ID, *warp.ID, xruntime.Location())
		}
		if len(warp.Destinations) == 0 {
			return errors.Errorf("场景传送点缺少目标: scene:%d warp:%d %v", *scene.ID, *warp.ID, xruntime.Location())
		}
		for destinationIndex, destination := range warp.Destinations {
			if destination.Condition == nil || strings.TrimSpace(*destination.Condition) == "" ||
				destination.Target == nil || destination.Target.MapID == nil ||
				destination.Target.X == nil || destination.Target.Y == nil ||
				!isSceneID(*destination.Target.MapID) {
				return errors.Errorf(
					"场景传送目标无效: scene:%d warp:%d index:%d %v",
					*scene.ID, *warp.ID, destinationIndex, xruntime.Location(),
				)
			}
		}
	}
	return nil
}

func (p *SceneConfig) check() error {
	var result error
	p.Foreach(func(sceneID uint32, scene *SceneEntry) bool {
		rules := make([][]SceneEnemyGroupEntry, 0, len(scene.Encounter.Regions)+1)
		rules = append(rules, scene.Encounter.Default.EnemyGroups)
		for _, region := range scene.Encounter.Regions {
			rules = append(rules, region.EnemyGroups)
		}
		for _, groups := range rules {
			for _, group := range groups {
				if GGameConfig.Enemy.Get(*group.ID) == nil {
					result = errors.Errorf(
						"场景引用了未定义敌人组: scene:%d enemyGroup:%d %v",
						sceneID, *group.ID, xruntime.Location(),
					)
					return false
				}
			}
		}
		for _, warp := range scene.Warps {
			for _, destination := range warp.Destinations {
				target := destination.Target
				targetScene := p.Get(*target.MapID)
				if targetScene != nil && (*target.X >= *targetScene.Width || *target.Y >= *targetScene.Height) {
					result = errors.Errorf(
						"场景传送目标坐标越界: scene:%d warp:%d targetScene:%d x:%d y:%d %v",
						sceneID, *warp.ID, *target.MapID, *target.X, *target.Y, xruntime.Location(),
					)
					return false
				}
			}
		}
		return true
	})
	return result
}

func (p *SceneConfig) assemble() error {
	return nil
}

func (p *SceneEntry) IsCoordinateValid(x uint32, y uint32) bool {
	return p != nil && p.Width != nil && p.Height != nil && x < *p.Width && y < *p.Height
}

func (p *SceneEntry) IsBlocked(x uint32, y uint32) bool {
	if !p.IsCoordinateValid(x, y) || p.Collision == nil {
		return true
	}
	return sceneRowsContain(p.Collision.BlockedRows, x, y)
}

func (p *SceneEntry) EncounterRuleAt(x uint32, y uint32) *SceneEncounterRuleEntry {
	if !p.IsCoordinateValid(x, y) || p.IsBlocked(x, y) || p.Encounter == nil {
		return nil
	}
	for _, region := range p.Encounter.Regions {
		if region != nil && sceneRowsContain(region.Rows, x, y) {
			return &SceneEncounterRuleEntry{
				Enabled:     region.Enabled,
				EnemyGroups: region.EnemyGroups,
			}
		}
	}
	return p.Encounter.Default
}

func sceneRowsContain(rows [][]uint32, x uint32, y uint32) bool {
	for _, row := range rows {
		if len(row) == 3 && row[0] == y && x >= row[1] && x <= row[2] {
			return true
		}
	}
	return false
}

func sceneCellKey(x uint32, y uint32) uint64 {
	return uint64(y)<<32 | uint64(x)
}
