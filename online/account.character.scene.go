package main

import (
	"server/common/gameconfig"
	pb "server/proto/pb"
)

func (p *Account) getCharacterScene(character *pb.CharacterRecord) (sceneEntry *gameconfig.SceneEntry) {
	sceneEntry = gameconfig.GGameConfig.Scene.Get(character.GetSceneId())
	return sceneEntry
}
