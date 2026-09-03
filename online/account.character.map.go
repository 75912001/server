package main

import (
	"errors"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onCharacterMapEnterReq(gateway *Gateway, packet *pb.OnlineClientPacket) {
	var request pb.CharacterMapEnterReq
	if err := proto.Unmarshal(packet.GetBody(), &request); err != nil ||
		request.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(request.GetCharacterUuid())
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	base := character.record.GetBase()
	if character.sceneID == request.GetMapId() {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.AlreadyExists.Code())
		return
	}
	if request.GetMapId() != 0 {
		if !isCharacterMapID(request.GetMapId()) {
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.InvalidArgument.Code())
			return
		}
		targetScene := gameconfig.GGameConfig.Scene.Get(request.GetMapId())
		if targetScene == nil {
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.NotFound.Code())
			return
		}
		if !characterMapEncounterEnabled(targetScene) {
			p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
	}

	key := sceneCharacterKey{aid: p.aid, characterUUID: base.GetUuid()}
	sourcePresence, ok := p.characterPresence(gateway, character)
	if !ok {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.Internal.Code())
		return
	}
	GCharacterTeamMgr.sequenceMu.Lock()
	defer GCharacterTeamMgr.sequenceMu.Unlock()

	members := GCharacterTeamMgr.orderedMembers(key)
	if request.GetMapId() == 0 && len(members) > 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if request.GetMapId() != 0 && len(members) > 0 && members[0].key != key {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	sourcePresences, err := characterMapEntryPresences(sourcePresence, members)
	if err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	targetPresences := make([]sceneCharacterPresence, len(sourcePresences))
	for index, presence := range sourcePresences {
		target := presence
		target.sceneID = request.GetMapId()
		targetPresences[index] = target
	}

	if !p.removeCharacterMapEntrySource(sourcePresences) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.Internal.Code())
		return
	}
	if request.GetMapId() == 0 {
		p.dispatchCharacterTeamSceneState(targetPresences[0])
		if character.autoEncounterEnabled {
			character.autoEncounterEnabled = false
			character.clearAutoEncounterTimer()
			character.notifyAutoEncounterState(gateway)
		}
		p.sendCharacterMapPacket(targetPresences[0], &pb.CharacterMapEnterRes{
			CharacterUuid: key.characterUUID,
			MapId:         0,
		})
		return
	}
	existing, joined := GScenePresenceMgr.joinCharacterMap(targetPresences)
	if !joined {
		xlog.GLog.Errorf("join character map failed aid:%d character:%d map:%d", p.aid, key.characterUUID, request.GetMapId())
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterMapEnterRes_CMD), xerror.Internal.Code())
		return
	}
	for _, presence := range targetPresences {
		p.dispatchCharacterTeamSceneState(presence)
	}
	groupList := characterMapGroups(existing)
	joinedGroup := characterMapGroup(targetPresences)
	for _, viewer := range existing {
		p.sendCharacterMapPacket(viewer, newCharacterMapJoinEvent(viewer, joinedGroup))
	}
	for _, presence := range targetPresences {
		p.sendCharacterMapPacket(presence, &pb.CharacterMapEnterRes{
			CharacterUuid: presence.key.characterUUID,
			MapId:         request.GetMapId(),
			GroupList:     groupList,
		})
	}
}

func characterMapEntryPresences(
	source sceneCharacterPresence,
	members []characterTeamMember,
) ([]sceneCharacterPresence, error) {
	if len(members) == 0 {
		if source.inCombat {
			return nil, errors.New("solo character is in combat")
		}
		if source.sceneID == 0 {
			return []sceneCharacterPresence{source}, nil
		}
		presence, ok := GScenePresenceMgr.get(source.sceneID, source.key)
		if !ok || presence.inCombat {
			return nil, errors.New("solo character presence is unavailable")
		}
		return []sceneCharacterPresence{presence}, nil
	}
	if source.sceneID == 0 {
		return nil, errors.New("team cannot exist outside maps")
	}
	result := make([]sceneCharacterPresence, 0, len(members))
	for _, member := range members {
		presence, ok := GScenePresenceMgr.get(source.sceneID, member.key)
		if !ok || presence.inCombat {
			return nil, errors.New("team member presence is unavailable")
		}
		result = append(result, presence)
	}
	return result, nil
}

func (p *Account) removeCharacterMapEntrySource(presences []sceneCharacterPresence) bool {
	if len(presences) == 0 {
		return false
	}
	if presences[0].sceneID == 0 {
		return true
	}
	if isCharacterMapID(presences[0].sceneID) {
		keys := make([]sceneCharacterKey, 0, len(presences))
		for _, presence := range presences {
			keys = append(keys, presence.key)
		}
		_, viewers, ok := GScenePresenceMgr.removeCharacterMap(presences[0].sceneID, keys)
		if !ok {
			return false
		}
		for _, viewer := range viewers {
			p.sendCharacterMapPacket(viewer, newCharacterMapLeaveEvent(viewer, keys[0]))
		}
		return true
	}
	for _, presence := range presences {
		_, ok := GScenePresenceMgr.remove(presence.sceneID, presence.key)
		if !ok {
			return false
		}
	}
	return true
}

func (p *Account) removeCharacterMapPresence(sceneID uint32, key sceneCharacterKey) bool {
	_, viewers, ok := GScenePresenceMgr.removeCharacterMap(sceneID, []sceneCharacterKey{key})
	if !ok {
		return false
	}
	for _, viewer := range viewers {
		p.sendCharacterMapPacket(viewer, newCharacterMapLeaveEvent(viewer, key))
	}
	return true
}
