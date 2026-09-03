package main

import pb "server/proto/pb"

// combatPoisonDamage复刻8.5 Compute_Down, 只读取中毒者的基础四维和当前HP.
// 角色档案保存整点, 宠物档案保存100倍固定点. 必须先求和再还原, 不能分别
// 截断四项小数, 也不能套用物理减伤、暴击、随机伤害或最大HP百分比.
func combatPoisonDamage(state *combatUnitRuntimeState) uint64 {
	if state == nil || state.hp <= 1 {
		return 0
	}
	total := state.rawVitality + state.rawStrength + state.rawToughness + state.rawDexterity
	if !combatUnitIsCharacter(state.unit) {
		total /= 100
	}
	damage := (total - 20) / 4
	if damage < 1 {
		damage = 1
	}
	return min(uint64(damage), state.hp-1)
}

// combatPoisonThreshold复刻BATTLE_StatusAttackCheck的普通毒分支.
// 当前战斗均为PVE: 等级差乘2后限制在[-40,40], 基础偏移30, 扣除目标毒抗
// 和体力占比*40, 最后只设80上限. 原版各阶段使用C float并在赋给per时截断;
// RAND(1,100)严格小于per才成功, 因此per=80实际对应79个成功结果.
func combatPoisonThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	total := defender.rawVitality + defender.rawStrength + defender.rawToughness + defender.rawDexterity
	if total == 0 {
		// 原版此输入会除零, 没有有效概率. 明确拒绝附毒, 不凭面板属性补造四维.
		return 0
	}
	vitalShare := float32(defender.rawVitality) / float32(total)
	vitalPenalty := float32(float64(vitalShare) / 0.25)
	vitalPenalty = float32(float64(vitalPenalty) * 10.0)
	levelModifier := (int64(attacker.unit.GetAttribute().GetLevel()) - int64(defender.unit.GetAttribute().GetLevel())) * 2
	levelModifier = max(int64(-40), min(int64(40), levelModifier))
	threshold := int64(float32(30+levelModifier+combatEffectiveLuck(attacker)-defender.poisonResistance) - vitalPenalty)
	return min(int64(80), threshold)
}

// tryInflictCombatPoison只在主动攻击造成正伤害后调用. 已中毒时不刷新计数,
// 也不消费额外随机数. 致死伤害仍保留原版概率抽取顺序, 但不向倒下目标挂状态.
func (r *CombatRoom) tryInflictCombatPoison(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, durationActions uint32, step *combatStepResult) {
	if defender.poisonTurns > 0 {
		return
	}
	if r.random.rangeInt(1, 100) >= combatPoisonThreshold(attacker, defender) {
		return
	}
	if !defender.alive || defender.escaped {
		return
	}
	defender.poisonTurns = durationActions
	combatAppendEffect(step, &combatEffectResult{
		EffectKind:        combatEffectKindStatus,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey: cloneCombatUnitKey(defender.unit.GetKey()),
			StatusDeltaList: []*pb.CombatStatusDelta{{
				StatusType: pb.CombatStatusType_CombatStatusType_Poison,
				DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Add,
				DurationAfter: &pb.CombatDuration{
					Unit:      pb.CombatDurationUnit_CombatDurationUnit_Action,
					Remaining: defender.poisonTurns,
				},
			}},
		}},
	})
}

// processCombatPoisonBeforeAction在正常行动前结算毒伤, 最后一次同时移除状态.
// 每名合击成员只结算一次, 连续攻击各段和反击不调用. 状态步骤先于本人的实际
// 动作, 通过现有Status cause与HP/status delta下发, 客户端不自行扣血或计数.
func (r *CombatRoom) processCombatPoisonBeforeAction(action *combatAction, steps *[]*combatStepResult) {
	state := r.stateByKey(action.unitKey)
	if state == nil || !state.alive || state.escaped || state.poisonTurns == 0 {
		return
	}
	before := state.hp
	damage := combatPoisonDamage(state)
	state.hp -= damage
	state.poisonTurns--
	statusDelta := &pb.CombatStatusDelta{
		StatusType: pb.CombatStatusType_CombatStatusType_Poison,
		DeltaType:  pb.CombatStatusDeltaType_CombatStatusDeltaType_Remove,
	}
	if state.poisonTurns > 0 {
		statusDelta.DeltaType = pb.CombatStatusDeltaType_CombatStatusDeltaType_Update
		statusDelta.DurationAfter = &pb.CombatDuration{
			Unit:      pb.CombatDurationUnit_CombatDurationUnit_Action,
			Remaining: state.poisonTurns,
		}
	}
	// HP=1时仍发送0伤害, 并照常消耗一次毒伤结算次数.
	unitDelta := &pb.CombatUnitStateDelta{
		UnitKey: cloneCombatUnitKey(state.unit.GetKey()),
		AssetDeltaList: []*pb.CombatAssetDelta{{
			AssetType: pb.CombatAssetType_CombatAssetType_HP,
			Delta:     combatClampDelta(damage),
			After:     combatClampUint32(state.hp),
		}},
		StatusDeltaList: []*pb.CombatStatusDelta{statusDelta},
	}
	effect := &combatEffectResult{
		EffectKind:        combatEffectKindDamage,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
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
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		EffectList:        []*combatEffectResult{effect},
	})
}

// resetRoundPoisonAttackModifiers对应下一回合BATTLE_TurnParam恢复攻击力.
// 清理的是施放者本回合的攻击命令修正, 不清理目标跨回合的中毒状态.
func (r *CombatRoom) resetRoundPoisonAttackModifiers() {
	for _, state := range r.unitStates {
		state.roundAttackPercentModifier = 0
	}
}
