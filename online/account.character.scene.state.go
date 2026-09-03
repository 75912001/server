package main

func (p *Account) removeCharacterPresence(sceneID uint32, key sceneCharacterKey) {
	if sceneID == 0 {
		return
	}
	if isCharacterMapID(sceneID) {
		p.removeCharacterMapPresence(sceneID, key)
		return
	}
	GScenePresenceMgr.remove(sceneID, key)
}

func (p *Account) refreshCharacterPresence(character *character) {
	presence, ok := p.characterPresence(nil, character)
	if !ok {
		return
	}
	if isCharacterMapID(presence.sceneID) {
		p.refreshCharacterMapPresence(presence)
		return
	}
	GScenePresenceMgr.upsert(presence)
}

func (p *Account) characterPresence(gateway *Gateway, character *character) (sceneCharacterPresence, bool) {
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		return sceneCharacterPresence{}, false
	}
	base := character.record.GetBase()
	gatewayKey := p.gatewayKey
	if gateway != nil {
		gatewayKey = gateway.Key
	}
	return sceneCharacterPresence{
		key: sceneCharacterKey{
			aid:           p.aid,
			characterUUID: base.GetUuid(),
		},
		gatewayKey:   gatewayKey,
		assetID:      base.GetAssetId(),
		nick:         base.GetNick(),
		exp:          base.GetExp(),
		rebirthCount: base.GetRebirthCount(),
		mountPetID:   characterMountedPetID(character.record),
		sceneID:      character.sceneID,
		teamEnabled:  character.teamEnabled,
		inCombat:     character.combatRoom != nil,
	}, true
}
