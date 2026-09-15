package main

import pb "server/proto/pb"

// tryInflictCombatDeepPoison复用普通状态攻击判定, 状态8没有独立面板抗性时按0处理.
func (r *CombatRoom) tryInflictCombatDeepPoison(attacker, defender *combatUnitRuntimeState, durationActions uint32, step *combatStepResult) {
	r.tryInflictCombatOrdinaryStatus(attacker, defender, durationActions, pb.CombatStatusType_CombatStatusType_DeepPoison, step)
}

// processCombatDeepPoisonBeforeAction复刻原版CHAR_WORKDEEPPOISON. 状态先递减;
// 剩余1时强制致死并取消本次行动, 其余阶段复用普通毒的四维伤害且至少保留1HP.
func (r *CombatRoom) processCombatDeepPoisonBeforeAction(action *combatAction, steps *[]*combatStepResult) bool {
	state := r.stateByKey(action.unitKey)
	if state == nil || !state.alive || state.escaped {
		return false
	}
	remaining := state.statusTurns[pb.CombatStatusType_CombatStatusType_DeepPoison]
	if remaining == 0 {
		return false
	}
	remaining--
	state.statusTurns[pb.CombatStatusType_CombatStatusType_DeepPoison] = remaining
	if state.hp <= 1 || remaining <= 1 {
		r.appendCombatDeepPoisonDeath(state, steps)
		return true
	}

	before := state.hp
	damage := combatPoisonDamage(state)
	state.hp -= damage
	unitKey := cloneCombatUnitKey(state.unit.GetKey())
	unitDelta := &pb.CombatUnitStateDelta{
		UnitKey: unitKey,
		AssetDeltaList: []*pb.CombatAssetDelta{{
			AssetType: pb.CombatAssetType_CombatAssetType_HP,
			Delta:     combatClampDelta(damage),
			After:     combatClampUint32(state.hp),
		}},
		StatusDeltaList: []*pb.CombatStatusDelta{{
			StatusType: pb.CombatStatusType_CombatStatusType_DeepPoison,
			DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Update,
			DurationAfter: &pb.CombatDuration{
				Unit:      pb.CombatDurationUnit_CombatDurationUnit_Action,
				Remaining: remaining,
			},
		}},
	}
	effect := &combatEffectResult{
		EffectKind:        combatEffectKindDamage,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		UnitDeltaList:     []*pb.CombatUnitStateDelta{unitDelta},
		Damage: &combatDamageDetail{
			DisplayedDamage: combatClampUint32(damage),
			AppliedHpDamage: combatClampUint32(damage),
			HpBefore:        combatClampUint32(before),
			HpAfter:         combatClampUint32(state.hp),
			HitResultList:   []combatHitResult{combatHitResultNormal},
		},
	}
	*steps = append(*steps, &combatStepResult{
		EventKind:         combatStepKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		EffectList:        []*combatEffectResult{effect},
	})
	return false
}

func (r *CombatRoom) appendCombatDeepPoisonDeath(state *combatUnitRuntimeState, steps *[]*combatStepResult) {
	before := state.hp
	damage := before
	state.hp = 0
	state.alive = false
	state.guard = false
	clearCombatContinuedActionState(state)
	clearCombatNoGuardState(state)

	statusDeltas := make([]*pb.CombatStatusDelta, 0, len(state.statusTurns)+1)
	if state.poisonTurns > 0 {
		state.poisonTurns = 0
		statusDeltas = append(statusDeltas, &pb.CombatStatusDelta{
			StatusType: pb.CombatStatusType_CombatStatusType_Poison,
			DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Remove,
		})
	}
	for statusType, remaining := range state.statusTurns {
		if remaining == 0 && statusType != pb.CombatStatusType_CombatStatusType_DeepPoison {
			continue
		}
		statusDeltas = append(statusDeltas, &pb.CombatStatusDelta{
			StatusType: statusType,
			DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Remove,
		})
	}
	clear(state.statusTurns)

	unitKey := cloneCombatUnitKey(state.unit.GetKey())
	step := &combatStepResult{
		EventKind:         combatStepKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
	}
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindDamage,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(unitKey)},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey: unitKey,
			AssetDeltaList: []*pb.CombatAssetDelta{{
				AssetType: pb.CombatAssetType_CombatAssetType_HP,
				Delta:     combatClampDelta(damage),
				After:     0,
			}},
			AliveChanged:    true,
			Alive:           false,
			StatusDeltaList: statusDeltas,
		}},
		Damage: &combatDamageDetail{
			DisplayedDamage: combatClampUint32(damage),
			AppliedHpDamage: combatClampUint32(damage),
			HpBefore:        combatClampUint32(before),
			HpAfter:         0,
			HitResultList:   []combatHitResult{combatHitResultNormal},
		},
	})
	r.appendCombatDefeatEffects(step, state, state, combatDamageApplication{
		hpBefore:      before,
		hpAfter:       0,
		appliedDamage: damage,
		killed:        true,
	}, false)
	*steps = append(*steps, step)
}
