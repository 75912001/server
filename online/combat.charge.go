package main

import pb "server/proto/pb"

// combatChargeState保留原版S_CHARGE的跨回合指令, 参数在首次实际行动时冻结.
// remainingRounds=0仍为待释放状态, 只有charge指针为空才表示可以重新选择技能.
type combatChargeState struct {
	skillID               uint32
	targetKey             *pb.CombatUnitKey
	remainingRounds       uint32
	attackPercentModifier int32
}

// continuedCombatChargeAction只读取在场单位的蓄力快照, 不重新读取配置或抽取AI技能及目标.
// 调用方通过requiredPlayerUnitKeys或敌方在场检查保证state及unit有效.
func continuedCombatChargeAction(state *combatUnitRuntimeState) *combatAction {
	if state.charge == nil {
		return nil
	}
	return &combatAction{
		unitKey:   cloneCombatUnitKey(state.unit.GetKey()),
		kind:      combatActionKindChargeAttack,
		skillID:   state.charge.skillID,
		targetKey: cloneCombatUnitKey(state.charge.targetKey),
	}
}

// pendingChargePlayerUnitKeys随本回合战报告知下一回合无需重新选招的玩家单位.
// 使用稳定的参与者顺序, 并排除死亡和离场单位; 释放前的零计数仍必须锁定.
func (r *CombatRoom) pendingChargePlayerUnitKeys() []*pb.CombatUnitKey {
	var keys []*pb.CombatUnitKey
	for _, key := range r.requiredPlayerUnitKeys() {
		if r.stateByKey(key).charge != nil {
			keys = append(keys, key)
		}
	}
	return keys
}

// combatChargeAttackPower对应BATTLE_Charge的pow += pow * Per * 0.01.
// 保留整数乘法、C double加法和最终向零截断的顺序, 不能复用猛毒的C float减攻公式.
func combatChargeAttackPower(attack int64, modifier int32) int64 {
	return int64(float64(attack) + float64(attack*int64(modifier))*0.01)
}

// executeChargeAttack在每次实际行动时推进一次计数, 蓄力本身仅产生无效果动作步骤.
// 原版先检查旧计数再递减, 所以1回合蓄力在第二次行动释放, 2回合蓄力在第三次释放.
func (r *CombatRoom) executeChargeAttack(action *combatAction, events *[]*combatStepResult) combatAttackOutcome {
	state := r.stateByKey(action.unitKey)
	if state.charge == nil {
		state.charge = &combatChargeState{
			skillID:               action.skillID,
			targetKey:             cloneCombatUnitKey(action.targetKey),
			remainingRounds:       action.chargeRounds,
			attackPercentModifier: action.chargeAttackPercentModifier,
		}
	}
	charge := state.charge
	if charge.remainingRounds > 0 {
		charge.remainingRounds--
		appendCombatActionOnlyStep(action, events)
		return combatAttackOutcome{}
	}

	// 释放时读取当前基础攻击力, 目标失效则由普通物理入口执行一次正常换目标.
	// 清除续招状态但保留Charge动作种类, 对应原版CHARGE_OK转NONE后的禁止自身反击资格.
	state.charge = nil
	attack := combatChargeAttackPower(int64(state.unit.GetAttribute().GetAttack()), charge.attackPercentModifier)
	state.chargeAttackPower = &attack
	defer func() { state.chargeAttackPower = nil }()
	return r.executeSingleAttack(action, false, events)
}
