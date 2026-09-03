package main

import "sync"

type sceneCharacterKey struct {
	aid           uint64
	characterUUID uint64
}

// sceneCharacterPresence 是场景成员索引持有的角色表现值副本.
// Account actor 仍独占角色档案; 场景只复制成员校验和广播必需字段, 不跨 actor 读取可变档案.
type sceneCharacterPresence struct {
	key          sceneCharacterKey
	gatewayKey   string
	assetID      uint64
	nick         string
	exp          uint64
	rebirthCount uint64
	mountPetID   uint64
	sceneID      uint32
	teamEnabled  bool
	inCombat     bool
}

// scenePresenceManager 按地图 ID 定位独立场景成员索引.
// 每张地图拥有自己的锁和角色表现索引, 地图 0 不创建索引.
type scenePresenceManager struct {
	mu      sync.RWMutex
	byScene map[uint32]*scenePresence
}

type scenePresence struct {
	mu       sync.RWMutex
	byKey    map[sceneCharacterKey]sceneCharacterPresence
	mapOrder []sceneCharacterKey
}

var GScenePresenceMgr = newScenePresenceManager()

func newScenePresenceManager() *scenePresenceManager {
	return &scenePresenceManager{byScene: make(map[uint32]*scenePresence)}
}

func newScenePresence() *scenePresence {
	return &scenePresence{byKey: make(map[sceneCharacterKey]sceneCharacterPresence)}
}

func (m *scenePresenceManager) scene(sceneID uint32, create bool) *scenePresence {
	if sceneID == 0 {
		return nil
	}
	m.mu.RLock()
	scene := m.byScene[sceneID]
	m.mu.RUnlock()
	if scene != nil || !create {
		return scene
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if scene = m.byScene[sceneID]; scene == nil {
		scene = newScenePresence()
		m.byScene[sceneID] = scene
	}
	return scene
}

// upsert 用于首次加入场景或更新角色表现字段.
func (m *scenePresenceManager) upsert(presence sceneCharacterPresence) bool {
	if !validSceneCharacterPresence(presence) {
		return false
	}
	scene := m.scene(presence.sceneID, true)
	scene.mu.Lock()
	defer scene.mu.Unlock()
	scene.upsertLocked(presence)
	return true
}

func (m *scenePresenceManager) remove(sceneID uint32, key sceneCharacterKey) (sceneCharacterPresence, bool) {
	scene := m.scene(sceneID, false)
	if scene == nil {
		return sceneCharacterPresence{}, false
	}
	scene.mu.Lock()
	defer scene.mu.Unlock()
	presence, ok := scene.byKey[key]
	if !ok {
		return sceneCharacterPresence{}, false
	}
	delete(scene.byKey, key)
	return presence, true
}

func (m *scenePresenceManager) get(sceneID uint32, key sceneCharacterKey) (sceneCharacterPresence, bool) {
	scene := m.scene(sceneID, false)
	if scene == nil {
		return sceneCharacterPresence{}, false
	}
	scene.mu.RLock()
	defer scene.mu.RUnlock()
	presence, ok := scene.byKey[key]
	return presence, ok
}

func validSceneCharacterPresence(presence sceneCharacterPresence) bool {
	return presence.key.aid != 0 && presence.key.characterUUID != 0 && presence.sceneID != 0 && presence.gatewayKey != ""
}

func (s *scenePresence) upsertLocked(presence sceneCharacterPresence) {
	s.byKey[presence.key] = presence
}
