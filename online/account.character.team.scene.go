package main

import (
	"context"

	xactor "github.com/75912001/xlib/actor"
)

func (p *Account) postCharacterTeamSceneState(presence sceneCharacterPresence) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineAccountActorCmdTeamScene, presence))
}

func (p *Account) applyCharacterTeamSceneState(presence sceneCharacterPresence) {
	character := p.characterManager.find(presence.key.characterUUID)
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		return
	}
	character.sceneID = presence.sceneID
}

func (p *Account) dispatchCharacterTeamSceneState(presence sceneCharacterPresence) {
	account := GAccountMgr.GetByAID(presence.key.aid)
	if account == nil {
		return
	}
	if account == p {
		p.applyCharacterTeamSceneState(presence)
		return
	}
	account.postCharacterTeamSceneState(presence)
}
