package main

import pb "server/proto/pb"

func isCharacterTestMapID(mapID uint32) bool {
	return mapID >= uint32(pb.AssetIDRange_AssetIDRange_Map_Test_Start) &&
		mapID <= uint32(pb.AssetIDRange_AssetIDRange_Map_Test_End)
}

func isCharacterTrainingMapID(mapID uint32) bool {
	return mapID >= uint32(pb.AssetIDRange_AssetIDRange_Map_Training_Start) &&
		mapID <= uint32(pb.AssetIDRange_AssetIDRange_Map_Training_End)
}

func isCharacterMapID(mapID uint32) bool {
	return isCharacterTestMapID(mapID) || isCharacterTrainingMapID(mapID)
}

// joinCharacterMap 在单张可进入地图锁内先取得现有角色副本, 再按完整分组顺序追加新角色.
func (m *scenePresenceManager) joinCharacterMap(presences []sceneCharacterPresence) ([]sceneCharacterPresence, bool) {
	if len(presences) == 0 || !isCharacterMapID(presences[0].sceneID) {
		return nil, false
	}
	sceneID := presences[0].sceneID
	seen := make(map[sceneCharacterKey]struct{}, len(presences))
	for _, presence := range presences {
		if !validSceneCharacterPresence(presence) || presence.sceneID != sceneID {
			return nil, false
		}
		if _, exists := seen[presence.key]; exists {
			return nil, false
		}
		seen[presence.key] = struct{}{}
	}

	scene := m.scene(sceneID, true)
	scene.mu.Lock()
	defer scene.mu.Unlock()
	for key := range seen {
		if _, exists := scene.byKey[key]; exists {
			return nil, false
		}
	}
	current := scene.characterMapPresencesLocked()
	for _, presence := range presences {
		scene.byKey[presence.key] = presence
		scene.mapOrder = append(scene.mapOrder, presence.key)
	}
	return current, true
}

// removeCharacterMap 在删除全部目标角色后返回剩余角色副本.
// keys 第一项同时是 map_leave 事件使用的第一层节点身份.
func (m *scenePresenceManager) removeCharacterMap(
	sceneID uint32,
	keys []sceneCharacterKey,
) ([]sceneCharacterPresence, []sceneCharacterPresence, bool) {
	if !isCharacterMapID(sceneID) || len(keys) == 0 {
		return nil, nil, false
	}
	scene := m.scene(sceneID, false)
	if scene == nil {
		return nil, nil, false
	}
	removeSet := make(map[sceneCharacterKey]struct{}, len(keys))
	scene.mu.Lock()
	defer scene.mu.Unlock()
	removed := make([]sceneCharacterPresence, 0, len(keys))
	for _, key := range keys {
		if _, exists := removeSet[key]; exists {
			return nil, nil, false
		}
		presence, exists := scene.byKey[key]
		if !exists {
			return nil, nil, false
		}
		removeSet[key] = struct{}{}
		removed = append(removed, presence)
	}
	nextOrder := make([]sceneCharacterKey, 0, len(scene.mapOrder)-len(removeSet))
	for _, key := range scene.mapOrder {
		if _, removing := removeSet[key]; removing {
			delete(scene.byKey, key)
			continue
		}
		nextOrder = append(nextOrder, key)
	}
	scene.mapOrder = nextOrder
	return removed, scene.characterMapPresencesLocked(), true
}

func (m *scenePresenceManager) characterMapPresences(sceneID uint32) []sceneCharacterPresence {
	if !isCharacterMapID(sceneID) {
		return nil
	}
	scene := m.scene(sceneID, false)
	if scene == nil {
		return nil
	}
	scene.mu.RLock()
	defer scene.mu.RUnlock()
	return scene.characterMapPresencesLocked()
}

// refreshCharacterMap 只替换可进入地图中的角色显示副本, 不改变第一层展示顺序.
func (m *scenePresenceManager) refreshCharacterMap(
	presence sceneCharacterPresence,
) (sceneCharacterPresence, []sceneCharacterPresence, bool) {
	if !validSceneCharacterPresence(presence) || !isCharacterMapID(presence.sceneID) {
		return sceneCharacterPresence{}, nil, false
	}
	scene := m.scene(presence.sceneID, false)
	if scene == nil {
		return sceneCharacterPresence{}, nil, false
	}
	scene.mu.Lock()
	defer scene.mu.Unlock()
	previous, exists := scene.byKey[presence.key]
	if !exists {
		return sceneCharacterPresence{}, nil, false
	}
	scene.byKey[presence.key] = presence
	viewers := make([]sceneCharacterPresence, 0, len(scene.mapOrder)-1)
	for _, key := range scene.mapOrder {
		if key == presence.key {
			continue
		}
		if viewer, ok := scene.byKey[key]; ok {
			viewers = append(viewers, viewer)
		}
	}
	return previous, viewers, true
}

// reorderCharacterMapToEnd 让离队或解散后形成的新第一层节点与客户端追加规则保持一致.
func (m *scenePresenceManager) reorderCharacterMapToEnd(
	sceneID uint32,
	keys []sceneCharacterKey,
) ([]sceneCharacterPresence, bool) {
	if !isCharacterMapID(sceneID) || len(keys) == 0 {
		return nil, false
	}
	scene := m.scene(sceneID, false)
	if scene == nil {
		return nil, false
	}
	moving := make(map[sceneCharacterKey]struct{}, len(keys))
	scene.mu.Lock()
	defer scene.mu.Unlock()
	for _, key := range keys {
		if _, duplicated := moving[key]; duplicated {
			return nil, false
		}
		if _, exists := scene.byKey[key]; !exists {
			return nil, false
		}
		moving[key] = struct{}{}
	}
	nextOrder := make([]sceneCharacterKey, 0, len(scene.mapOrder))
	for _, key := range scene.mapOrder {
		if _, move := moving[key]; !move {
			nextOrder = append(nextOrder, key)
		}
	}
	nextOrder = append(nextOrder, keys...)
	scene.mapOrder = nextOrder
	return scene.characterMapPresencesLocked(), true
}

func (s *scenePresence) characterMapPresencesLocked() []sceneCharacterPresence {
	result := make([]sceneCharacterPresence, 0, len(s.mapOrder))
	for _, key := range s.mapOrder {
		if presence, exists := s.byKey[key]; exists {
			result = append(result, presence)
		}
	}
	return result
}
