package main

import (
	"errors"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	"google.golang.org/protobuf/proto"
)

func isCharacterTeamOperationRequestValid(request *pb.CharacterTeamOperationReq) bool {
	if request == nil || request.GetCharacterUuid() == 0 {
		return false
	}
	switch operation := request.GetOperation().(type) {
	case *pb.CharacterTeamOperationReq_Join:
		target := operation.Join.GetTarget()
		return operation.Join != nil && target != nil && target.GetAid() > 0 && target.GetCharacterUuid() > 0
	case *pb.CharacterTeamOperationReq_Leave:
		return operation.Leave != nil
	case *pb.CharacterTeamOperationReq_Disband:
		return operation.Disband != nil
	case *pb.CharacterTeamOperationReq_Kick:
		target := operation.Kick.GetTarget()
		return operation.Kick != nil && target != nil && target.GetAid() > 0 && target.GetCharacterUuid() > 0
	default:
		return false
	}
}

func (p *Account) onCharacterTeamOperationReq(gateway *Gateway, packet *pb.OnlineClientPacket) {
	var request pb.CharacterTeamOperationReq
	if err := proto.Unmarshal(packet.GetBody(), &request); err != nil || !isCharacterTeamOperationRequestValid(&request) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterTeamOperationRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(request.GetCharacterUuid())
	if character == nil || character.record == nil || character.record.GetBase() == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterTeamOperationRes_CMD), xerror.NotFound.Code())
		return
	}
	_, leavingTeam := request.GetOperation().(*pb.CharacterTeamOperationReq_Leave)
	// 主动离队只改变队伍关系. 已进入独立 CombatRoom 的参战成员和本场战斗不随队伍变化重建.
	if !character.online || (character.combatRoom != nil && !leavingTeam) {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterTeamOperationRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	base := character.record.GetBase()
	key := sceneCharacterKey{aid: p.aid, characterUUID: base.GetUuid()}
	GCharacterTeamMgr.sequenceMu.Lock()
	defer GCharacterTeamMgr.sequenceMu.Unlock()
	var mutation characterTeamMutation
	var operationErr error
	response := &pb.CharacterTeamOperationRes{CharacterUuid: base.GetUuid()}
	joinOperation := false
	switch operation := request.GetOperation().(type) {
	case *pb.CharacterTeamOperationReq_Join:
		joinOperation = true
		response.Operation = &pb.CharacterTeamOperationRes_Join{Join: &pb.CharacterTeamJoinRes{}}
		target := operation.Join.GetTarget()
		targetKey := sceneCharacterKey{aid: target.GetAid(), characterUUID: target.GetCharacterUuid()}
		mutation, operationErr = GCharacterTeamMgr.join(
			character.sceneID,
			key,
			targetKey,
		)
	case *pb.CharacterTeamOperationReq_Leave:
		response.Operation = &pb.CharacterTeamOperationRes_Leave{Leave: &pb.CharacterTeamLeaveRes{}}
		mutation, operationErr = GCharacterTeamMgr.leave(key)
	case *pb.CharacterTeamOperationReq_Disband:
		response.Operation = &pb.CharacterTeamOperationRes_Disband{Disband: &pb.CharacterTeamDisbandRes{}}
		mutation, operationErr = GCharacterTeamMgr.disband(key)
	case *pb.CharacterTeamOperationReq_Kick:
		response.Operation = &pb.CharacterTeamOperationRes_Kick{Kick: &pb.CharacterTeamKickRes{}}
		target := operation.Kick.GetTarget()
		targetKey := sceneCharacterKey{aid: target.GetAid(), characterUUID: target.GetCharacterUuid()}
		mutation, operationErr = GCharacterTeamMgr.kick(
			key,
			targetKey,
		)
	default:
		operationErr = errCharacterTeamInvalidArgument
	}
	if operationErr != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterTeamOperationRes_CMD), characterTeamResultID(operationErr))
		return
	}
	if joinOperation && character.autoEncounterEnabled {
		character.autoEncounterEnabled = false
		character.clearAutoEncounterTimer()
		character.notifyAutoEncounterState(gateway)
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterTeamOperationRes_CMD), xerror.Success.Code(), response)
	p.applyCharacterTeamMutation(mutation)
}

func characterTeamResultID(err error) uint32 {
	switch {
	case errors.Is(err, errCharacterTeamInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errCharacterTeamNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errCharacterTeamFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errCharacterTeamResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}

func (p *Account) dischargeCharacterTeam(key sceneCharacterKey) {
	GCharacterTeamMgr.coordinate(func() {
		p.applyCharacterTeamMutation(GCharacterTeamMgr.remove(key))
	})
}

// removeFailedCombatAdmissionMember 将仍在线但无法绑定本场战斗的普通成员移出原队伍.
// 条件移除和地图/角色通知共用队伍变更串行边界, 不会影响已经离队或换队的角色.
func (p *Account) removeFailedCombatAdmissionMember(leaderKey sceneCharacterKey, memberKey sceneCharacterKey) {
	GCharacterTeamMgr.coordinate(func() {
		mutation, removed := GCharacterTeamMgr.removeMemberIfLedBy(leaderKey, memberKey)
		if removed {
			p.applyCharacterTeamMutation(mutation)
		}
	})
}

func (p *Account) applyCharacterTeamMutation(mutation characterTeamMutation) {
	p.applyCharacterMapTeamEvent(mutation.mapEvent)
	for _, member := range mutation.notifications {
		target := sceneCharacterPresence{key: member.key, gatewayKey: member.gatewayKey}
		presence, ok := GScenePresenceMgr.find(member.key)
		if ok {
			target = presence
		}
		p.sendScenePresencePacket(
			target,
			uint32(pb.MsgID_CharacterTeamChangedNotify_CMD),
			xerror.Success.Code(),
			&pb.CharacterTeamChangedNotify{
				TargetCharacterUuid: member.key.characterUUID,
			},
		)
	}
}

// combatResultDischargesCharacterTeam 只识别原版会解除队伍的成功逃跑和 Ultimate 击飞.
// 普通倒地虽然也可能包含 alive=false, 但不属于解除队伍条件.
func combatResultDischargesCharacterTeam(result *pb.CombatRoundResultNotify, key sceneCharacterKey) bool {
	if result == nil || key.aid == 0 || key.characterUUID == 0 {
		return false
	}
	for _, event := range result.GetEventList() {
		for _, step := range event.GetActionStepList() {
			for _, effect := range step.GetEffectList() {
				if !combatUnitKeyListContainsCharacter(effect.GetAffectedUnitKeyList(), key) {
					continue
				}
				if effect.GetKnockback() != nil {
					return true
				}
				if escape := effect.GetEscape(); escape != nil && escape.GetSuccess() {
					return true
				}
			}
		}
	}
	return false
}

func combatUnitKeyListContainsCharacter(unitKeyList []*pb.CombatUnitKey, key sceneCharacterKey) bool {
	for _, unitKey := range unitKeyList {
		if unitKey.GetAid() == key.aid &&
			unitKey.GetCharacterUuid() == key.characterUUID &&
			unitKey.GetPetUuid() == 0 {
			return true
		}
	}
	return false
}

func combatResultContainsCharacterUnitLeave(
	result *pb.CombatRoundResultNotify,
	key sceneCharacterKey,
	reason pb.CombatUnitLeaveReason,
) bool {
	if result == nil || key.aid == 0 || key.characterUUID == 0 || reason == pb.CombatUnitLeaveReason_CombatUnitLeaveReason_Unknown {
		return false
	}
	for _, event := range result.GetEventList() {
		for _, step := range event.GetActionStepList() {
			for _, effect := range step.GetEffectList() {
				if effect.GetUnitLeave().GetReason() == reason &&
					combatUnitKeyListContainsCharacter(effect.GetAffectedUnitKeyList(), key) {
					return true
				}
			}
		}
	}
	return false
}
