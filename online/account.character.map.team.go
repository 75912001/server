package main

import pb "server/proto/pb"

func (p *Account) applyCharacterMapTeamEvent(event *characterTeamMapEvent) {
	if event == nil || len(event.orderKeys) == 0 {
		return
	}
	source, ok := GScenePresenceMgr.find(event.key)
	if !ok || !isCharacterMapID(source.sceneID) {
		return
	}
	viewers, ok := GScenePresenceMgr.reorderCharacterMapToEnd(source.sceneID, event.orderKeys)
	if !ok {
		return
	}
	var joinedGroup *pb.CharacterMapGroup
	if event.eventType == characterTeamMapEventJoin {
		joinedGroup = characterMapGroupByKeys(viewers, event.orderKeys)
		if joinedGroup == nil {
			return
		}
	}
	for _, viewer := range viewers {
		switch event.eventType {
		case characterTeamMapEventJoin:
			p.sendCharacterMapPacket(viewer, newCharacterMapTeamJoinEvent(viewer, joinedGroup))
		case characterTeamMapEventLeave:
			p.sendCharacterMapPacket(viewer, newCharacterMapTeamLeaveEvent(viewer, event.key))
		case characterTeamMapEventDisband:
			p.sendCharacterMapPacket(viewer, newCharacterMapTeamDisbandEvent(viewer, event.key))
		}
	}
}

// characterMapGroupByKeys 从同一张地图的权威 Presence 快照按队伍顺序构造完整分组.
// 任一成员缺失时拒绝广播,避免客户端收到不完整的 team_join 后错误覆盖旧名单.
func characterMapGroupByKeys(
	presences []sceneCharacterPresence,
	keys []sceneCharacterKey,
) *pb.CharacterMapGroup {
	byKey := make(map[sceneCharacterKey]sceneCharacterPresence, len(presences))
	for _, presence := range presences {
		byKey[presence.key] = presence
	}
	ordered := make([]sceneCharacterPresence, 0, len(keys))
	for _, key := range keys {
		presence, ok := byKey[key]
		if !ok {
			return nil
		}
		ordered = append(ordered, presence)
	}
	if len(ordered) < 2 {
		return nil
	}
	return characterMapGroup(ordered)
}
