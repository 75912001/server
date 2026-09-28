package main

import pb "server/proto/pb"

// combatHealingSpiritAmount复刻原版RAND基础浮动与体力回复倍率, 最终按整数截断.
func (r *CombatRoom) combatHealingSpiritAmount(target *combatUnitRuntimeState, power uint32) uint64 {
	if target == nil || power == 0 {
		return 0
	}
	rolled := r.random.rangeInt(int64(power)*9/10, int64(power)*11/10)
	multiplier := 1.0 + float64(target.rawVitality)*0.00005
	if combatUnitIsPlayerCharacter(target.unit) {
		multiplier = 1.0 + float64(target.rawVitality)*0.01
	}
	return uint64(float64(rolled) * multiplier)
}

func combatHealingUnitDelta(deltasByToken map[string]*pb.CombatUnitStateDelta, ordered *[]*pb.CombatUnitStateDelta, key *pb.CombatUnitKey) *pb.CombatUnitStateDelta {
	token := combatUnitKeyMapKey(key)
	if existing := deltasByToken[token]; existing != nil {
		return existing
	}
	delta := &pb.CombatUnitStateDelta{UnitKey: cloneCombatUnitKey(key)}
	deltasByToken[token] = delta
	*ordered = append(*ordered, delta)
	return delta
}

func (r *CombatRoom) executeHealingSpirit(action *combatAction, events *[]*combatStepResult) {
	attacker := r.stateByKey(action.unitKey)
	if attacker == nil || !attacker.alive || attacker.escaped || attacker.mp < uint64(action.healMPCost) {
		return
	}
	targetKeys := []*pb.CombatUnitKey{cloneCombatUnitKey(action.targetKey)}
	if action.healTargetScope == "allyCamp" {
		targetKeys = r.aliveAllyKeys(action.unitKey)
	}
	if len(targetKeys) == 0 {
		return
	}

	attacker.mp -= uint64(action.healMPCost)
	unitDeltas := make([]*pb.CombatUnitStateDelta, 0, len(targetKeys)+1)
	deltasByToken := make(map[string]*pb.CombatUnitStateDelta, len(targetKeys)+1)
	sourceDelta := combatHealingUnitDelta(deltasByToken, &unitDeltas, action.unitKey)
	sourceDelta.AssetDeltaList = append(sourceDelta.AssetDeltaList, &pb.CombatAssetDelta{
		AssetType: pb.CombatAssetType_CombatAssetType_MP,
		Delta:     combatClampDecreaseDelta(uint64(action.healMPCost)),
		After:     combatClampUint32(attacker.mp),
	})

	for _, targetKey := range targetKeys {
		target := r.stateByKey(targetKey)
		if target == nil || !target.alive || target.escaped || target.hp >= target.maxHP {
			continue
		}
		recovered := r.combatHealingSpiritAmount(target, action.healPower)
		if remaining := target.maxHP - target.hp; recovered > remaining {
			recovered = remaining
		}
		if recovered == 0 {
			continue
		}
		target.hp += recovered
		targetDelta := combatHealingUnitDelta(deltasByToken, &unitDeltas, targetKey)
		targetDelta.AssetDeltaList = append(targetDelta.AssetDeltaList, &pb.CombatAssetDelta{
			AssetType: pb.CombatAssetType_CombatAssetType_HP,
			Delta:     combatClampIncreaseDelta(recovered),
			After:     combatClampUint32(target.hp),
		})
	}

	step := &combatStepResult{
		EventKind:         combatStepKindAction,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: cloneCombatUnitKeyList(targetKeys),
	}
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindHeal,
		SourceUnitKeyList: cloneCombatUnitKeyList(step.SourceUnitKeyList),
		TargetUnitKeyList: cloneCombatUnitKeyList(targetKeys),
		UnitDeltaList:     unitDeltas,
	})
	*events = append(*events, step)
}
