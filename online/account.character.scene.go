package main

import "server/common/gameconfig"

func (p *Account) getCharacterScene(character *character) (sceneEntry *gameconfig.SceneEntry) {
	sceneEntry = gameconfig.GGameConfig.Scene.Get(character.sceneID)
	return sceneEntry
}
