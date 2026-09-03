package main

import (
	"errors"
	"sync"
)

const characterTeamMemberLimit = 5

var (
	errCharacterTeamInvalidArgument    = errors.New("character team invalid argument")
	errCharacterTeamNotFound           = errors.New("character team not found")
	errCharacterTeamFailedPrecondition = errors.New("character team failed precondition")
	errCharacterTeamResourceExhausted  = errors.New("character team resource exhausted")
)

type characterTeamMember struct {
	key        sceneCharacterKey
	gatewayKey string
}

// characterTeam 使用紧凑成员列表保存队伍顺序. 第一个成员始终是队长,
// 普通成员离队后后续成员直接前移, 新成员只追加到末尾.
type characterTeam struct {
	members []characterTeamMember
}

type characterTeamMapEventType uint8

const (
	characterTeamMapEventJoin characterTeamMapEventType = iota + 1
	characterTeamMapEventLeave
	characterTeamMapEventDisband
)

type characterTeamMapEvent struct {
	eventType characterTeamMapEventType
	key       sceneCharacterKey
	orderKeys []sceneCharacterKey
}

type characterTeamMutation struct {
	notifications []characterTeamMember
	mapEvent      *characterTeamMapEvent
}

type characterTeamManager struct {
	sequenceMu sync.Mutex
	mu         sync.RWMutex
	byMember   map[sceneCharacterKey]*characterTeam
}

var GCharacterTeamMgr = newCharacterTeamManager()

func newCharacterTeamManager() *characterTeamManager {
	return &characterTeamManager{byMember: make(map[sceneCharacterKey]*characterTeam)}
}

func (m *characterTeamManager) coordinate(action func()) {
	m.sequenceMu.Lock()
	defer m.sequenceMu.Unlock()
	action()
}

func (m *characterTeamManager) membership(key sceneCharacterKey) (bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	team := m.byMember[key]
	if team == nil || len(team.members) == 0 {
		return false, false
	}
	return true, team.members[0].key == key
}

func (m *characterTeamManager) orderedMembers(key sceneCharacterKey) []characterTeamMember {
	m.mu.RLock()
	defer m.mu.RUnlock()
	team := m.byMember[key]
	if team == nil || len(team.members) == 0 {
		return nil
	}
	members := make([]characterTeamMember, len(team.members))
	copy(members, team.members)
	return members
}

func (m *characterTeamManager) sameTeam(left sceneCharacterKey, right sceneCharacterKey) bool {
	if left == right {
		return true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	team := m.byMember[left]
	return team != nil && team == m.byMember[right]
}

func (m *characterTeamManager) join(sceneID uint32, sourceKey sceneCharacterKey, targetKey sceneCharacterKey) (characterTeamMutation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source, target, err := m.joinCandidateLocked(sceneID, sourceKey, targetKey)
	if err != nil {
		return characterTeamMutation{}, err
	}

	sourceMember := newCharacterTeamMember(source)
	targetTeam := m.byMember[targetKey]
	if targetTeam == nil {
		targetMember := newCharacterTeamMember(target)
		targetTeam = &characterTeam{members: []characterTeamMember{targetMember, sourceMember}}
		m.byMember[targetKey] = targetTeam
		m.byMember[sourceKey] = targetTeam
		return characterTeamMutation{
			notifications: characterTeamNotifications(targetTeam),
			mapEvent:      newCharacterTeamJoinMapEvent(targetTeam),
		}, nil
	}
	if len(targetTeam.members) >= characterTeamMemberLimit {
		return characterTeamMutation{}, errCharacterTeamResourceExhausted
	}
	targetTeam.members = append(targetTeam.members, sourceMember)
	m.byMember[sourceKey] = targetTeam
	return characterTeamMutation{
		notifications: characterTeamNotifications(targetTeam),
		mapEvent:      newCharacterTeamJoinMapEvent(targetTeam),
	}, nil
}

func (m *characterTeamManager) leave(key sceneCharacterKey) (characterTeamMutation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	team := m.byMember[key]
	if team == nil {
		return characterTeamMutation{}, errCharacterTeamNotFound
	}
	memberIndex := characterTeamMemberIndex(team, key)
	if memberIndex <= 0 {
		return characterTeamMutation{}, errCharacterTeamFailedPrecondition
	}
	return m.removeMemberLocked(team, memberIndex), nil
}

func (m *characterTeamManager) disband(key sceneCharacterKey) (characterTeamMutation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	team := m.byMember[key]
	if team == nil {
		return characterTeamMutation{}, errCharacterTeamNotFound
	}
	if len(team.members) == 0 || team.members[0].key != key {
		return characterTeamMutation{}, errCharacterTeamFailedPrecondition
	}
	return m.disbandLocked(team), nil
}

func (m *characterTeamManager) kick(leaderKey sceneCharacterKey, targetKey sceneCharacterKey) (characterTeamMutation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	team := m.byMember[leaderKey]
	if team == nil {
		return characterTeamMutation{}, errCharacterTeamNotFound
	}
	if len(team.members) == 0 || team.members[0].key != leaderKey {
		return characterTeamMutation{}, errCharacterTeamFailedPrecondition
	}
	targetIndex := characterTeamMemberIndex(team, targetKey)
	if targetIndex <= 0 {
		return characterTeamMutation{}, errCharacterTeamInvalidArgument
	}
	return m.removeMemberLocked(team, targetIndex), nil
}

// removeMemberIfLedBy 只在两个角色仍属于同一支队伍且 leaderKey 仍是队长时移除普通成员.
// 该条件把遇敌候选快照与当前队伍状态重新对齐, 防止绑定失败处理误删已经换队的角色.
func (m *characterTeamManager) removeMemberIfLedBy(leaderKey sceneCharacterKey, memberKey sceneCharacterKey) (characterTeamMutation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	team := m.byMember[leaderKey]
	if team == nil || len(team.members) == 0 || team.members[0].key != leaderKey || m.byMember[memberKey] != team {
		return characterTeamMutation{}, false
	}
	memberIndex := characterTeamMemberIndex(team, memberKey)
	if memberIndex <= 0 {
		return characterTeamMutation{}, false
	}
	return m.removeMemberLocked(team, memberIndex), true
}

// remove 用于下线、成功逃跑和 Ultimate 击飞等强制生命周期.
// 队长离开时解散整队, 普通成员离开时保持剩余成员的紧凑顺序.
func (m *characterTeamManager) remove(key sceneCharacterKey) characterTeamMutation {
	m.mu.Lock()
	defer m.mu.Unlock()
	team := m.byMember[key]
	if team == nil {
		return characterTeamMutation{}
	}
	memberIndex := characterTeamMemberIndex(team, key)
	if memberIndex == 0 {
		return m.disbandLocked(team)
	}
	if memberIndex > 0 {
		return m.removeMemberLocked(team, memberIndex)
	}
	return characterTeamMutation{}
}

func (m *characterTeamManager) joinCandidateLocked(
	sceneID uint32,
	sourceKey sceneCharacterKey,
	targetKey sceneCharacterKey,
) (sceneCharacterPresence, sceneCharacterPresence, error) {
	if sceneID == 0 ||
		sourceKey.aid == 0 || sourceKey.characterUUID == 0 ||
		targetKey.aid == 0 || targetKey.characterUUID == 0 ||
		sourceKey == targetKey {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamInvalidArgument
	}
	if m.byMember[sourceKey] != nil {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamFailedPrecondition
	}
	source, sourceExists := GScenePresenceMgr.get(sceneID, sourceKey)
	target, targetExists := GScenePresenceMgr.get(sceneID, targetKey)
	if !sourceExists || !targetExists {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamNotFound
	}
	if source.inCombat {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamFailedPrecondition
	}
	if target.inCombat || !target.teamEnabled {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamNotFound
	}
	targetTeam := m.byMember[targetKey]
	if targetTeam != nil && (len(targetTeam.members) == 0 || targetTeam.members[0].key != targetKey) {
		return sceneCharacterPresence{}, sceneCharacterPresence{}, errCharacterTeamNotFound
	}
	return source, target, nil
}

func (m *characterTeamManager) removeMemberLocked(team *characterTeam, memberIndex int) characterTeamMutation {
	removed := team.members[memberIndex]
	copy(team.members[memberIndex:], team.members[memberIndex+1:])
	team.members = team.members[:len(team.members)-1]
	delete(m.byMember, removed.key)

	if len(team.members) == 1 {
		leader := team.members[0]
		delete(m.byMember, leader.key)
		team.members = nil
		return characterTeamMutation{
			notifications: []characterTeamMember{leader, removed},
			mapEvent: &characterTeamMapEvent{
				eventType: characterTeamMapEventLeave,
				key:       removed.key,
				orderKeys: []sceneCharacterKey{removed.key},
			},
		}
	}
	notifications := characterTeamNotifications(team)
	notifications = append(notifications, removed)
	return characterTeamMutation{
		notifications: notifications,
		mapEvent: &characterTeamMapEvent{
			eventType: characterTeamMapEventLeave,
			key:       removed.key,
			orderKeys: []sceneCharacterKey{removed.key},
		},
	}
}

func (m *characterTeamManager) disbandLocked(team *characterTeam) characterTeamMutation {
	if team == nil || len(team.members) == 0 {
		return characterTeamMutation{}
	}
	leaderKey := team.members[0].key
	notifications := characterTeamNotifications(team)
	orderKeys := make([]sceneCharacterKey, 0, len(team.members))
	for _, member := range team.members {
		orderKeys = append(orderKeys, member.key)
		delete(m.byMember, member.key)
	}
	team.members = nil
	return characterTeamMutation{
		notifications: notifications,
		mapEvent: &characterTeamMapEvent{
			eventType: characterTeamMapEventDisband,
			key:       leaderKey,
			orderKeys: orderKeys,
		},
	}
}

// newCharacterTeamJoinMapEvent 固化合并后的完整成员顺序. 地图 Presence 会按该顺序
// 把新队伍移动到末尾,所有观察者再用同一个 team_join 事件原子替换旧分组.
func newCharacterTeamJoinMapEvent(team *characterTeam) *characterTeamMapEvent {
	if team == nil || len(team.members) < 2 {
		return nil
	}
	orderKeys := make([]sceneCharacterKey, 0, len(team.members))
	for _, member := range team.members {
		orderKeys = append(orderKeys, member.key)
	}
	return &characterTeamMapEvent{
		eventType: characterTeamMapEventJoin,
		key:       orderKeys[0],
		orderKeys: orderKeys,
	}
}

func newCharacterTeamMember(presence sceneCharacterPresence) characterTeamMember {
	return characterTeamMember{
		key:        presence.key,
		gatewayKey: presence.gatewayKey,
	}
}

func characterTeamMemberIndex(team *characterTeam, key sceneCharacterKey) int {
	if team == nil {
		return -1
	}
	for i, member := range team.members {
		if member.key == key {
			return i
		}
	}
	return -1
}

func characterTeamNotifications(team *characterTeam) []characterTeamMember {
	if team == nil || len(team.members) == 0 {
		return nil
	}
	notifications := make([]characterTeamMember, len(team.members))
	copy(notifications, team.members)
	return notifications
}
