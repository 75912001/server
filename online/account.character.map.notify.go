package main

import (
	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	"google.golang.org/protobuf/proto"
)

func (p *Account) sendCharacterMapPacket(target sceneCharacterPresence, message proto.Message) {
	messageID := uint32(pb.MsgID_CharacterMapEventNotify_CMD)
	if _, response := message.(*pb.CharacterMapEnterRes); response {
		messageID = uint32(pb.MsgID_CharacterMapEnterRes_CMD)
	}
	p.sendScenePresencePacket(target, messageID, xerror.Success.Code(), message)
}

func newCharacterMapJoinEvent(
	viewer sceneCharacterPresence,
	group *pb.CharacterMapGroup,
) *pb.CharacterMapEventNotify {
	return &pb.CharacterMapEventNotify{
		TargetCharacterUuid: viewer.key.characterUUID,
		Event:               &pb.CharacterMapEventNotify_MapJoin{MapJoin: group},
	}
}

func newCharacterMapTeamJoinEvent(
	viewer sceneCharacterPresence,
	group *pb.CharacterMapGroup,
) *pb.CharacterMapEventNotify {
	return &pb.CharacterMapEventNotify{
		TargetCharacterUuid: viewer.key.characterUUID,
		Event:               &pb.CharacterMapEventNotify_TeamJoin{TeamJoin: group},
	}
}

func newCharacterMapLeaveEvent(
	viewer sceneCharacterPresence,
	key sceneCharacterKey,
) *pb.CharacterMapEventNotify {
	return &pb.CharacterMapEventNotify{
		TargetCharacterUuid: viewer.key.characterUUID,
		Event: &pb.CharacterMapEventNotify_MapLeave{MapLeave: &pb.CharacterKey{
			Aid:           key.aid,
			CharacterUuid: key.characterUUID,
		}},
	}
}

func newCharacterMapTeamLeaveEvent(
	viewer sceneCharacterPresence,
	key sceneCharacterKey,
) *pb.CharacterMapEventNotify {
	return &pb.CharacterMapEventNotify{
		TargetCharacterUuid: viewer.key.characterUUID,
		Event: &pb.CharacterMapEventNotify_TeamLeave{TeamLeave: &pb.CharacterKey{
			Aid:           key.aid,
			CharacterUuid: key.characterUUID,
		}},
	}
}

func newCharacterMapTeamDisbandEvent(
	viewer sceneCharacterPresence,
	key sceneCharacterKey,
) *pb.CharacterMapEventNotify {
	return &pb.CharacterMapEventNotify{
		TargetCharacterUuid: viewer.key.characterUUID,
		Event: &pb.CharacterMapEventNotify_TeamDisband{TeamDisband: &pb.CharacterKey{
			Aid:           key.aid,
			CharacterUuid: key.characterUUID,
		}},
	}
}

func (p *Account) refreshCharacterMapPresence(presence sceneCharacterPresence) bool {
	previous, viewers, ok := GScenePresenceMgr.refreshCharacterMap(presence)
	if !ok {
		return false
	}
	if !characterMapVisibleInfoChanged(previous, presence) {
		return true
	}
	info := mapCharacterInfo(presence)
	for _, viewer := range viewers {
		if GCharacterTeamMgr.sameTeam(presence.key, viewer.key) {
			continue
		}
		p.sendCharacterMapPacket(viewer, &pb.CharacterMapEventNotify{
			TargetCharacterUuid: viewer.key.characterUUID,
			Event:               &pb.CharacterMapEventNotify_CharacterUpdate{CharacterUpdate: info},
		})
	}
	return true
}

func characterMapVisibleInfoChanged(previous sceneCharacterPresence, current sceneCharacterPresence) bool {
	if previous.assetID != current.assetID ||
		previous.nick != current.nick ||
		previous.rebirthCount != current.rebirthCount ||
		previous.mountPetID != current.mountPetID ||
		previous.teamEnabled != current.teamEnabled ||
		previous.inCombat != current.inCombat {
		return true
	}
	if previous.exp == current.exp {
		return false
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Exp == nil {
		return true
	}
	previousLevel, previousErr := gameconfig.GGameConfig.Exp.GetLevel(previous.exp)
	currentLevel, currentErr := gameconfig.GGameConfig.Exp.GetLevel(current.exp)
	return previousErr != nil || currentErr != nil || previousLevel != currentLevel
}

func characterMapGroups(presences []sceneCharacterPresence) []*pb.CharacterMapGroup {
	byKey := make(map[sceneCharacterKey]sceneCharacterPresence, len(presences))
	for _, presence := range presences {
		byKey[presence.key] = presence
	}
	seen := make(map[sceneCharacterKey]struct{}, len(presences))
	groups := make([]*pb.CharacterMapGroup, 0, len(presences))
	for _, presence := range presences {
		if _, exists := seen[presence.key]; exists {
			continue
		}
		members := GCharacterTeamMgr.orderedMembers(presence.key)
		if len(members) == 0 {
			seen[presence.key] = struct{}{}
			groups = append(groups, characterMapGroup([]sceneCharacterPresence{presence}))
			continue
		}
		groupPresences := make([]sceneCharacterPresence, 0, len(members))
		for _, member := range members {
			item, exists := byKey[member.key]
			if !exists {
				continue
			}
			seen[member.key] = struct{}{}
			groupPresences = append(groupPresences, item)
		}
		if len(groupPresences) > 0 {
			groups = append(groups, characterMapGroup(groupPresences))
		}
	}
	return groups
}

func characterMapGroup(presences []sceneCharacterPresence) *pb.CharacterMapGroup {
	group := &pb.CharacterMapGroup{CharacterList: make([]*pb.MapCharacterInfo, 0, len(presences))}
	for _, presence := range presences {
		group.CharacterList = append(group.CharacterList, mapCharacterInfo(presence))
	}
	return group
}

func mapCharacterInfo(presence sceneCharacterPresence) *pb.MapCharacterInfo {
	return &pb.MapCharacterInfo{
		Character: &pb.CharacterKey{
			Aid:           presence.key.aid,
			CharacterUuid: presence.key.characterUUID,
		},
		CharacterId:  presence.assetID,
		Nick:         presence.nick,
		RebirthCount: presence.rebirthCount,
		Exp:          presence.exp,
		MountPetId:   presence.mountPetID,
		TeamEnabled:  presence.teamEnabled,
		InCombat:     presence.inCombat,
	}
}

func characterMountedPetID(record *pb.CharacterRecord) uint64 {
	if record == nil {
		return 0
	}
	for _, pet := range record.GetPetRecordList() {
		if pet != nil && pet.GetCarryStatus() == pb.PetCarryStatus_PetCarryStatus_Mount {
			return uint64(pet.GetAssetId())
		}
	}
	return 0
}

func characterMapEncounterEnabled(scene *gameconfig.SceneEntry) bool {
	return scene != nil && scene.Encounter != nil && scene.Encounter.Enabled != nil &&
		*scene.Encounter.Enabled && len(scene.Encounter.EnemyGroups) > 0
}
