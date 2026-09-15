package main

import pb "server/proto/pb"

func combatStatusSpiritThreshold(attacker, defender *combatUnitRuntimeState, action *combatAction) int64 {
	if attacker == nil || defender == nil || action == nil || attacker.unit == nil || defender.unit == nil {
		return 0
	}
	total := defender.rawVitality + defender.rawStrength + defender.rawToughness + defender.rawDexterity
	if total == 0 {
		return 0
	}
	vitalShare := float32(defender.rawVitality) / float32(total)
	vitalPenalty := float32(float64(vitalShare)/0.25) * 10
	level := int64(attacker.unit.GetAttribute().GetLevel()) - int64(defender.unit.GetAttribute().GetLevel())
	rangeLimit := int64(action.statusLevelDifferenceRange)
	level = max(-rangeLimit, min(rangeLimit, level))
	resistance := defender.statusResistance[action.statusType]
	threshold := int64(float32(int64(action.statusBaseSuccess)+level+combatEffectiveLuck(attacker)-resistance) - vitalPenalty)
	return min(int64(80), threshold)
}

func combatStateHasAbnormalStatus(state *combatUnitRuntimeState) bool {
	if state == nil {
		return false
	}
	if state.poisonTurns > 0 {
		return true
	}
	for _, remaining := range state.statusTurns {
		if remaining > 0 {
			return true
		}
	}
	return false
}

func combatNoGuardBlocked(state *combatUnitRuntimeState) bool {
	if state == nil {
		return true
	}
	return state.statusTurns[pb.CombatStatusType_CombatStatusType_Sleep] > 0 ||
		state.statusTurns[pb.CombatStatusType_CombatStatusType_Stone] > 0
}

func (r *CombatRoom) executeStatusSpirit(action *combatAction, events *[]*combatStepResult) {
	attacker := r.stateByKey(action.unitKey)
	if attacker == nil || !attacker.alive || attacker.escaped || attacker.mp < uint64(action.statusMPCost) {
		return
	}
	attacker.mp -= uint64(action.statusMPCost)
	targetKeys := make([]*pb.CombatUnitKey, 0, 1)
	if action.statusTargetScope == "opponentCamp" {
		targetKeys = r.aliveOpponentKeys(action.unitKey)
	} else if target := r.resolveCombatTarget(action); target != nil {
		// 单体魔法和物理动作共用执行期目标调整, 避免同回合较早隐藏的
		// 地球一周单位仍被已经提交的敌对魔法命中.
		targetKeys = append(targetKeys, cloneCombatUnitKey(target.unit.GetKey()))
	}
	step := &combatStepResult{
		EventKind:         combatStepKindAction,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: cloneCombatUnitKeyList(targetKeys),
	}
	unitDeltas := []*pb.CombatUnitStateDelta{{
		UnitKey: cloneCombatUnitKey(action.unitKey),
		AssetDeltaList: []*pb.CombatAssetDelta{{
			AssetType: pb.CombatAssetType_CombatAssetType_MP,
			Delta:     combatClampDelta(uint64(action.statusMPCost)),
			After:     combatClampUint32(attacker.mp),
		}},
	}}
	for _, targetKey := range targetKeys {
		defender := r.stateByKey(targetKey)
		if defender == nil || !defender.alive || defender.escaped || combatStateHasAbnormalStatus(defender) {
			continue
		}
		if r.random.rangeInt(1, 100) >= combatStatusSpiritThreshold(attacker, defender, action) {
			continue
		}
		if action.statusType == pb.CombatStatusType_CombatStatusType_Poison {
			defender.poisonTurns = action.statusDurationActions
		} else {
			if defender.statusTurns == nil {
				defender.statusTurns = make(map[pb.CombatStatusType]uint32)
			}
			defender.statusTurns[action.statusType] = action.statusDurationActions
		}
		unitDeltas = append(unitDeltas, &pb.CombatUnitStateDelta{
			UnitKey: cloneCombatUnitKey(targetKey),
			StatusDeltaList: []*pb.CombatStatusDelta{{
				StatusType: action.statusType,
				DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Add,
				DurationAfter: &pb.CombatDuration{
					Unit:      pb.CombatDurationUnit_CombatDurationUnit_Action,
					Remaining: action.statusDurationActions,
				},
			}},
		})
	}
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: cloneCombatUnitKeyList(targetKeys),
		UnitDeltaList:     unitDeltas,
	})
	*events = append(*events, step)
}

// processCombatControlBeforeAction遵循原版顺序: 先按行动递减状态, 本次行动仍受递减前状态影响.
func (r *CombatRoom) processCombatControlBeforeAction(action *combatAction, events *[]*combatStepResult) bool {
	state := r.stateByKey(action.unitKey)
	if state == nil || !state.alive || state.escaped || len(state.statusTurns) == 0 {
		return false
	}
	blocked := state.statusTurns[pb.CombatStatusType_CombatStatusType_Sleep] > 0 ||
		state.statusTurns[pb.CombatStatusType_CombatStatusType_Stone] > 0
	statusDeltas := make([]*pb.CombatStatusDelta, 0, len(state.statusTurns))
	for statusType, remaining := range state.statusTurns {
		// 剧毒有独立的行动前扣血和到期致死流程, 不能在通用状态循环中再次递减.
		if statusType == pb.CombatStatusType_CombatStatusType_DeepPoison {
			continue
		}
		if remaining == 0 {
			continue
		}
		remaining--
		delta := &pb.CombatStatusDelta{StatusType: statusType, DeltaType: pb.CombatStatusDeltaType_CombatStatusDeltaType_Remove}
		if remaining > 0 {
			state.statusTurns[statusType] = remaining
			delta.DeltaType = pb.CombatStatusDeltaType_CombatStatusDeltaType_Update
			delta.DurationAfter = &pb.CombatDuration{Unit: pb.CombatDurationUnit_CombatDurationUnit_Action, Remaining: remaining}
		} else {
			delete(state.statusTurns, statusType)
		}
		statusDeltas = append(statusDeltas, delta)
	}
	if len(statusDeltas) > 0 {
		step := &combatStepResult{EventKind: combatStepKindStatus, SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)}, TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)}}
		combatAppendEffect(step, &combatEffectResult{EffectKind: combatEffectKindStatus, SourceUnitKeyList: cloneCombatUnitKeyList(step.SourceUnitKeyList), TargetUnitKeyList: cloneCombatUnitKeyList(step.TargetUnitKeyList), UnitDeltaList: []*pb.CombatUnitStateDelta{{UnitKey: cloneCombatUnitKey(action.unitKey), StatusDeltaList: statusDeltas}}})
		*events = append(*events, step)
	}
	if blocked {
		if state.guard {
			state.guard = false
		}
		clearCombatNoGuardState(state)
		return true
	}
	if state.statusTurns[pb.CombatStatusType_CombatStatusType_Confusion] > 0 && r.random.rangeInt(1, 100) <= 80 {
		camp := pb.CombatCamp_CombatCamp_Initiator
		if r.random.rangeInt(0, 1) != 0 {
			camp = pb.CombatCamp_CombatCamp_Defender
		}
		startPosition := uint32(r.random.rangeInt(0, 9))
		var targetKey *pb.CombatUnitKey
		for offset := uint32(1); offset <= 10; offset++ {
			candidate := r.combatStateAtPosition(camp, (startPosition+offset)%10)
			if candidate == nil || candidate.unit == nil || !candidate.alive || candidate.escaped || candidate.hidden ||
				combatUnitKeyEqual(candidate.unit.GetKey(), action.unitKey) {
				continue
			}
			targetKey = cloneCombatUnitKey(candidate.unit.GetKey())
			break
		}
		state.guard = false
		state.guardianProtectedUnitKey = nil
		clearCombatNoGuardState(state)
		action.kind = combatActionKindAttack
		action.skillID = combatSkillAttack
		action.targetKey = targetKey
		action.confusionRewritten = true
	}
	return false
}
