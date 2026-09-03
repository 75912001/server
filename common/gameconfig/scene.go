package gameconfig

import (
	"math"
	"os"
	"path/filepath"
	pb "server/proto/pb"
	"strconv"
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

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
	NPCs      []SceneNPCEntry      `yaml:"npcs"`
	Warps     []SceneWarpEntry     `yaml:"warps"`
}

type SceneNPCEntry struct {
	EntityID        *uint32                       `yaml:"entityId"`
	Name            *string                       `yaml:"name"`
	X               *uint32                       `yaml:"x"`
	Y               *uint32                       `yaml:"y"`
	FunctionOptions []SceneNPCFunctionOptionEntry `yaml:"functionOptions"`
}

type SceneNPCFunctionOptionEntry struct {
	OptionID   *uint32           `yaml:"optionId"`
	FunctionID *pb.NpcFunctionID `yaml:"functionId"`
	Name       *string           `yaml:"name"`
	Enabled    *bool             `yaml:"enabled"`
	Config     map[string]any    `yaml:"config"`
}

type SceneCollisionEntry struct {
	BlockedRows [][]uint32 `yaml:"blockedRows"`
}

type SceneEncounterEntry struct {
	Enabled     *bool                  `yaml:"enabled"`
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
	sceneDir := filepath.Join(dir, DirScene)
	files, err := os.ReadDir(sceneDir)
	if err != nil {
		return errors.Errorf("读取场景配置目录失败: %s err:%v", sceneDir, err)
	}
	loaded := 0
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yaml" {
			continue
		}
		filenameSceneID, parseErr := strconv.ParseUint(strings.TrimSuffix(file.Name(), ".yaml"), 10, 32)
		if parseErr != nil || !isSceneID(uint32(filenameSceneID)) {
			return errors.Errorf("场景配置文件名必须是有效地图ID: %s %v", file.Name(), xruntime.Location())
		}
		var root struct {
			Scenes []*SceneEntry `yaml:"scenes"`
		}
		if err := loadYAMLFile(sceneDir, file.Name(), &root); err != nil {
			return err
		}
		if len(root.Scenes) != 1 || root.Scenes[0] == nil || root.Scenes[0].ID == nil {
			return errors.Errorf("单地图场景配置必须且只能包含一个场景: file:%s %v", file.Name(), xruntime.Location())
		}
		if *root.Scenes[0].ID != uint32(filenameSceneID) {
			return errors.Errorf(
				"场景配置文件名与地图ID不一致: file:%s scene:%d %v",
				file.Name(), *root.Scenes[0].ID, xruntime.Location(),
			)
		}
		if err := p.configure(root.Scenes); err != nil {
			return errors.Wrapf(err, "file:%s", file.Name())
		}
		loaded++
	}
	if loaded == 0 {
		return errors.Errorf("场景配置目录没有地图文件: %s %v", sceneDir, xruntime.Location())
	}
	return nil
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
		if scene.Encounter == nil {
			return errors.Errorf("场景缺少 encounter: scene:%d %v", sceneID, xruntime.Location())
		}
		if err := validateSceneEncounter(sceneID, scene.Encounter); err != nil {
			return err
		}
		if scene.Warps == nil {
			return errors.Errorf("场景缺少 warps: scene:%d %v", sceneID, xruntime.Location())
		}
		if err := validateSceneNPCs(scene); err != nil {
			return err
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

func validateSceneEncounter(
	sceneID uint32,
	encounter *SceneEncounterEntry,
) error {
	if encounter.Enabled == nil {
		return errors.Errorf("场景遇敌配置缺少 enabled: scene:%d %v", sceneID, xruntime.Location())
	}
	groups := encounter.EnemyGroups
	if groups == nil {
		return errors.Errorf("场景遇敌配置缺少 enemyGroups: scene:%d %v", sceneID, xruntime.Location())
	}
	if *encounter.Enabled && len(groups) == 0 {
		return errors.Errorf("启用的场景遇敌配置 enemyGroups 不能为空: scene:%d %v", sceneID, xruntime.Location())
	}
	groupIDs := make(map[uint32]struct{}, len(groups))
	totalWeight := uint64(0)
	for index := range groups {
		group := &groups[index]
		if group.ID == nil || *group.ID == 0 {
			return errors.Errorf("场景遇敌配置敌人组ID无效: scene:%d index:%d %v", sceneID, index, xruntime.Location())
		}
		if _, exists := groupIDs[*group.ID]; exists {
			return errors.Errorf("场景遇敌配置敌人组ID重复: scene:%d group:%d %v", sceneID, *group.ID, xruntime.Location())
		}
		groupIDs[*group.ID] = struct{}{}
		if group.Weight == nil || *group.Weight > uint32(math.MaxInt32) {
			return errors.Errorf("场景遇敌配置权重无效: scene:%d group:%d %v", sceneID, *group.ID, xruntime.Location())
		}
		totalWeight += uint64(*group.Weight)
	}
	if len(groups) > 0 && (totalWeight == 0 || totalWeight > uint64(math.MaxInt32)) {
		return errors.Errorf(
			"场景遇敌配置总权重无效: scene:%d total:%d %v",
			sceneID, totalWeight, xruntime.Location(),
		)
	}
	return nil
}

func validateSceneNPCs(scene *SceneEntry) error {
	sceneID := *scene.ID
	entityIDs := make(map[uint32]struct{}, len(scene.NPCs))
	for npcIndex := range scene.NPCs {
		npc := &scene.NPCs[npcIndex]
		if npc.EntityID == nil || *npc.EntityID == 0 {
			return errors.Errorf("场景NPC实体ID无效: scene:%d index:%d %v", sceneID, npcIndex, xruntime.Location())
		}
		entityID := *npc.EntityID
		if _, exists := entityIDs[entityID]; exists {
			return errors.Errorf("场景NPC实体ID重复: scene:%d npc:%d %v", sceneID, entityID, xruntime.Location())
		}
		entityIDs[entityID] = struct{}{}
		if npc.Name == nil || strings.TrimSpace(*npc.Name) == "" {
			return errors.Errorf("场景NPC名称为空: scene:%d npc:%d %v", sceneID, entityID, xruntime.Location())
		}
		if npc.X == nil || npc.Y == nil || *npc.X >= *scene.Width || *npc.Y >= *scene.Height {
			return errors.Errorf("场景NPC坐标越界: scene:%d npc:%d %v", sceneID, entityID, xruntime.Location())
		}
		if npc.FunctionOptions == nil {
			return errors.Errorf("场景NPC缺少 functionOptions: scene:%d npc:%d %v", sceneID, entityID, xruntime.Location())
		}
		optionIDs := make(map[uint32]struct{}, len(npc.FunctionOptions))
		for optionIndex := range npc.FunctionOptions {
			option := &npc.FunctionOptions[optionIndex]
			if option.OptionID == nil || *option.OptionID == 0 {
				return errors.Errorf("场景NPC功能选项ID无效: scene:%d npc:%d index:%d %v", sceneID, entityID, optionIndex, xruntime.Location())
			}
			optionID := *option.OptionID
			if _, exists := optionIDs[optionID]; exists {
				return errors.Errorf("场景NPC功能选项ID重复: scene:%d npc:%d option:%d %v", sceneID, entityID, optionID, xruntime.Location())
			}
			optionIDs[optionID] = struct{}{}
			if option.FunctionID == nil || *option.FunctionID <= pb.NpcFunctionID_NpcFunctionID_Unspecified || *option.FunctionID >= pb.NpcFunctionID_NpcFunctionID_Max {
				return errors.Errorf("场景NPC功能ID无效: scene:%d npc:%d option:%d %v", sceneID, entityID, optionID, xruntime.Location())
			}
			if option.Enabled == nil {
				return errors.Errorf("场景NPC功能缺少enabled: scene:%d npc:%d option:%d %v", sceneID, entityID, optionID, xruntime.Location())
			}
			if *option.FunctionID == pb.NpcFunctionID_NpcFunctionID_LegacyUnverified && *option.Enabled {
				return errors.Errorf("未验证的原版NPC功能不允许启用: scene:%d npc:%d option:%d %v", sceneID, entityID, optionID, xruntime.Location())
			}
			if option.Name == nil || strings.TrimSpace(*option.Name) == "" || option.Config == nil {
				return errors.Errorf("场景NPC功能名称或config无效: scene:%d npc:%d option:%d %v", sceneID, entityID, optionID, xruntime.Location())
			}
			if *option.FunctionID == pb.NpcFunctionID_NpcFunctionID_BattleChallenge {
				if _, ok := option.BattleChallengeEnemyGroupID(); !ok || len(option.Config) != 1 {
					return errors.Errorf(
						"场景NPC挑战配置必须且只能包含有效enemyGroupId: scene:%d npc:%d option:%d %v",
						sceneID, entityID, optionID, xruntime.Location(),
					)
				}
			}
		}
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
		for _, group := range scene.Encounter.EnemyGroups {
			if GGameConfig.Enemy.Get(*group.ID) == nil {
				result = errors.Errorf(
					"场景引用了未定义敌人组: scene:%d enemyGroup:%d %v",
					sceneID, *group.ID, xruntime.Location(),
				)
				return false
			}
		}
		for npcIndex := range scene.NPCs {
			npc := &scene.NPCs[npcIndex]
			for optionIndex := range npc.FunctionOptions {
				option := &npc.FunctionOptions[optionIndex]
				if option.FunctionID == nil ||
					*option.FunctionID != pb.NpcFunctionID_NpcFunctionID_BattleChallenge ||
					option.Enabled == nil || !*option.Enabled {
					continue
				}
				enemyGroupID, ok := option.BattleChallengeEnemyGroupID()
				if !ok || GGameConfig.Enemy.Get(enemyGroupID) == nil {
					result = errors.Errorf(
						"场景NPC挑战引用了未定义敌人组: scene:%d npc:%d option:%d enemyGroup:%d %v",
						sceneID, *npc.EntityID, *option.OptionID, enemyGroupID, xruntime.Location(),
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

func (p *SceneEntry) NPCFunctionOption(entityID uint32, optionID uint32) (*SceneNPCFunctionOptionEntry, bool) {
	if p == nil || entityID == 0 || optionID == 0 {
		return nil, false
	}
	for npcIndex := range p.NPCs {
		npc := &p.NPCs[npcIndex]
		if npc.EntityID == nil || *npc.EntityID != entityID {
			continue
		}
		for optionIndex := range npc.FunctionOptions {
			option := &npc.FunctionOptions[optionIndex]
			if option.OptionID != nil && *option.OptionID == optionID {
				return option, true
			}
		}
		return nil, false
	}
	return nil, false
}

func (p *SceneNPCFunctionOptionEntry) BattleChallengeEnemyGroupID() (uint32, bool) {
	if p == nil || p.Config == nil {
		return 0, false
	}
	value, exists := p.Config["enemyGroupId"]
	if !exists {
		return 0, false
	}
	var enemyGroupID uint64
	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			return 0, false
		}
		enemyGroupID = uint64(typed)
	case int64:
		if typed <= 0 {
			return 0, false
		}
		enemyGroupID = uint64(typed)
	case uint:
		enemyGroupID = uint64(typed)
	case uint32:
		enemyGroupID = uint64(typed)
	case uint64:
		enemyGroupID = typed
	default:
		return 0, false
	}
	if enemyGroupID == 0 || enemyGroupID > math.MaxUint32 {
		return 0, false
	}
	return uint32(enemyGroupID), true
}

func (p *SceneEntry) IsBlocked(x uint32, y uint32) bool {
	if !p.IsCoordinateValid(x, y) || p.Collision == nil {
		return true
	}
	return sceneRowsContain(p.Collision.BlockedRows, x, y)
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
