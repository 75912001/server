package main

import pb "server/proto/pb"

// combatOrdinaryStatusAttackThreshold复刻BATTLE_StatusAttackCheck的普通异常攻击分支.
// PVE等级差乘2后限制在[-40,40], 再叠加基础30、施放者幸运、目标对应抗性
// 和目标基础体力占比惩罚. 原版只设80上限, RAND(1,100)严格小于阈值才成功.
func combatOrdinaryStatusAttackThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, statusType pb.CombatStatusType) int64 {
	if attacker == nil || defender == nil || attacker.unit == nil || defender.unit == nil {
		return 0
	}
	total := defender.rawVitality + defender.rawStrength + defender.rawToughness + defender.rawDexterity
	if total == 0 {
		return 0
	}
	vitalShare := float32(defender.rawVitality) / float32(total)
	vitalPenalty := float32(float64(vitalShare) / 0.25)
	vitalPenalty = float32(float64(vitalPenalty) * 10.0)
	levelModifier := (int64(attacker.unit.GetAttribute().GetLevel()) - int64(defender.unit.GetAttribute().GetLevel())) * 2
	levelModifier = max(int64(-40), min(int64(40), levelModifier))
	resistance := defender.statusResistance[statusType]
	threshold := int64(float32(30+levelModifier+combatEffectiveLuck(attacker)-resistance) - vitalPenalty)
	return min(int64(80), threshold)
}

func combatStoneThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	return combatOrdinaryStatusAttackThreshold(attacker, defender, pb.CombatStatusType_CombatStatusType_Stone)
}

func combatConfusionThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	return combatOrdinaryStatusAttackThreshold(attacker, defender, pb.CombatStatusType_CombatStatusType_Confusion)
}

func combatSleepThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	return combatOrdinaryStatusAttackThreshold(attacker, defender, pb.CombatStatusType_CombatStatusType_Sleep)
}

func combatAppendStatusRemoveEffect(step *combatStepResult, attacker, defender *combatUnitRuntimeState, statusType pb.CombatStatusType) {
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey: cloneCombatUnitKey(defender.unit.GetKey()),
			StatusDeltaList: []*pb.CombatStatusDelta{{
				StatusType: statusType,
				DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Remove,
			}},
		}},
	})
}

// wakeCombatSleepAfterPhysicalDamage统一处理正物理伤害唤醒. PoisonAttack主动段由调用方显式排除.
func (r *CombatRoom) wakeCombatSleepAfterPhysicalDamage(attacker, defender *combatUnitRuntimeState, step *combatStepResult) {
	if attacker == nil || defender == nil || step == nil || defender.statusTurns[pb.CombatStatusType_CombatStatusType_Sleep] == 0 {
		return
	}
	delete(defender.statusTurns, pb.CombatStatusType_CombatStatusType_Sleep)
	combatAppendStatusRemoveEffect(step, attacker, defender, pb.CombatStatusType_CombatStatusType_Sleep)
}

// tryInflictCombatOrdinaryStatus只在主动段造成正伤害后调用. 原版DamageWakeUp先解除Sleep,
// 再检查剩余普通异常; 只有完全无异常时才消费状态概率随机. 致死伤害仍消费
// 概率随机, 但不会向已经倒下或离场的目标写入新状态.
func (r *CombatRoom) tryInflictCombatOrdinaryStatus(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, durationActions uint32, statusType pb.CombatStatusType, step *combatStepResult) {
	if attacker == nil || defender == nil || step == nil {
		return
	}
	r.wakeCombatSleepAfterPhysicalDamage(attacker, defender, step)
	if combatStateHasAbnormalStatus(defender) {
		return
	}
	if r.random.rangeInt(1, 100) >= combatOrdinaryStatusAttackThreshold(attacker, defender, statusType) {
		return
	}
	if !defender.alive || defender.escaped {
		return
	}
	if defender.statusTurns == nil {
		defender.statusTurns = make(map[pb.CombatStatusType]uint32)
	}
	remaining := durationActions + 1
	defender.statusTurns[statusType] = remaining
	if statusType == pb.CombatStatusType_CombatStatusType_Stone || statusType == pb.CombatStatusType_CombatStatusType_Sleep {
		defender.guard = false
		defender.guardianProtectedUnitKey = nil
		clearCombatNoGuardState(defender)
	}
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey: cloneCombatUnitKey(defender.unit.GetKey()),
			StatusDeltaList: []*pb.CombatStatusDelta{{
				StatusType: statusType,
				DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Add,
				DurationAfter: &pb.CombatDuration{
					Unit:      pb.CombatDurationUnit_CombatDurationUnit_Action,
					Remaining: remaining,
				},
			}},
		}},
	})
}

func (r *CombatRoom) tryInflictCombatStone(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, durationActions uint32, step *combatStepResult) {
	r.tryInflictCombatOrdinaryStatus(attacker, defender, durationActions, pb.CombatStatusType_CombatStatusType_Stone, step)
}

func (r *CombatRoom) tryInflictCombatConfusion(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, durationActions uint32, step *combatStepResult) {
	r.tryInflictCombatOrdinaryStatus(attacker, defender, durationActions, pb.CombatStatusType_CombatStatusType_Confusion, step)
}

func (r *CombatRoom) tryInflictCombatSleep(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, durationActions uint32, step *combatStepResult) {
	r.tryInflictCombatOrdinaryStatus(attacker, defender, durationActions, pb.CombatStatusType_CombatStatusType_Sleep, step)
}
