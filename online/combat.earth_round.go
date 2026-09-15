package main

import pb "server/proto/pb"

// combatEarthRoundState保存原版EARTHROUND跨回合指令.
// 技能ID、目标、倍率和首次行动值一经隐藏动作执行即冻结, 不受配置热更新影响.
type combatEarthRoundState struct {
	skillID               uint32
	targetKey             *pb.CombatUnitKey
	damagePercentModifier int32
	actionValue           int64
}

// continuedCombatAction统一返回服务端锁定的跨回合动作. Charge优先级只用于防御
// 异常内存状态; 正常配置行为块互斥, 一个单位不会同时存在两种续招.
func continuedCombatAction(state *combatUnitRuntimeState) *combatAction {
	if state == nil || state.unit == nil {
		return nil
	}
	if action := continuedCombatChargeAction(state); action != nil {
		return action
	}
	if state.earthRound == nil {
		return nil
	}
	return &combatAction{
		unitKey:                         cloneCombatUnitKey(state.unit.GetKey()),
		kind:                            combatActionKindEarthRound,
		skillID:                         state.earthRound.skillID,
		targetKey:                       cloneCombatUnitKey(state.earthRound.targetKey),
		actionValue:                     state.earthRound.actionValue,
		actionValueFrozen:               true,
		earthRoundDamagePercentModifier: state.earthRound.damagePercentModifier,
		earthRoundRelease:               true,
	}
}

func clearCombatContinuedActionState(state *combatUnitRuntimeState) {
	if state == nil {
		return
	}
	state.charge = nil
	state.earthRound = nil
	state.hidden = false
}

func combatEarthRoundAdjustedDamage(damage uint64, modifier int32) uint64 {
	multiplier := float32(1) + float32(modifier)*0.01
	return uint64(int64(float32(damage) * multiplier))
}

func combatAppendVisibilityEffect(event *combatStepResult, state *combatUnitRuntimeState, hidden bool) {
	if event == nil || state == nil || state.unit == nil {
		return
	}
	key := cloneCombatUnitKey(state.unit.GetKey())
	combatAppendEffect(event, &combatEffectResult{
		EffectKind:        combatEffectKindVisibility,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(key)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(key)},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey:       cloneCombatUnitKey(key),
			HiddenChanged: true,
			Hidden:        hidden,
		}},
		Hidden: hidden,
	})
}

// executeEarthRound执行两阶段动作. 首次行动只隐藏; 自动续招先在运行态现身,
// 再由普通单段物理链输出visibility(false)和后续伤害、死亡、击飞及反击结果.
func (r *CombatRoom) executeEarthRound(action *combatAction, events *[]*combatStepResult) combatAttackOutcome {
	state := r.stateByKey(action.unitKey)
	if state == nil || state.unit == nil || !state.alive || state.escaped {
		return combatAttackOutcome{}
	}
	if !action.earthRoundRelease {
		state.earthRound = &combatEarthRoundState{
			skillID:               action.skillID,
			targetKey:             cloneCombatUnitKey(action.targetKey),
			damagePercentModifier: action.earthRoundDamagePercentModifier,
			actionValue:           action.actionValue,
		}
		state.hidden = true
		event := &combatStepResult{
			EventKind:         combatStepKindAction,
			SkillId:           action.skillID,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
			TargetUnitKeyList: cloneCombatUnitKeyList([]*pb.CombatUnitKey{action.targetKey}),
		}
		combatAppendVisibilityEffect(event, state, true)
		*events = append(*events, event)
		return combatAttackOutcome{}
	}

	state.earthRound = nil
	state.hidden = false
	before := len(*events)
	outcome := r.executeSingleAttack(action, false, events)
	if len(*events) == before {
		// 同回合的更快动作可能先清空全部敌人. 即使没有可执行的物理目标,
		// 也必须向客户端发布现身增量, 避免隐藏状态残留到战斗结算切场.
		event := &combatStepResult{
			EventKind:         combatStepKindAction,
			SkillId:           action.skillID,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		}
		combatAppendVisibilityEffect(event, state, false)
		*events = append(*events, event)
	}
	return outcome
}
