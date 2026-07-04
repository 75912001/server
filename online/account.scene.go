package main

import (
	"fmt"

	pb "server/proto/pb"
)

func (p *Account) characterSceneID(character *pb.CharacterRecord) (uint32, error) {
	if character == nil || character.GetUuid() == 0 {
		return 0, fmt.Errorf("character record invalid")
	}
	sceneID := character.GetSceneId()
	if !isValidSceneID(sceneID) {
		return 0, fmt.Errorf("scene not found: %d", sceneID)
	}
	return sceneID, nil
}

func isValidSceneID(sceneID uint32) bool {
	if sceneID == 0 || GGameConfig == nil || GGameConfig.Scene == nil {
		return false
	}
	return GGameConfig.Scene.GetByID(int(sceneID)) != nil
}
