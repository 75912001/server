package main

func (m *scenePresenceManager) find(key sceneCharacterKey) (sceneCharacterPresence, bool) {
	m.mu.RLock()
	scenes := make([]*scenePresence, 0, len(m.byScene))
	for _, scene := range m.byScene {
		scenes = append(scenes, scene)
	}
	m.mu.RUnlock()
	for _, scene := range scenes {
		scene.mu.RLock()
		presence, ok := scene.byKey[key]
		scene.mu.RUnlock()
		if ok {
			return presence, true
		}
	}
	return sceneCharacterPresence{}, false
}
