package main

import (
	"fmt"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
)

// combatCaptureChance 复刻 BATTLE_CaptureCheck 的 float 运算顺序. 原版使用当前 HP 的平方,
// 阈值只封顶 99, 不保底; 调用方必须用 RAND(1,100) 严格小于阈值判定.
// 本版本尚无睡眠或捕获加成状态, 因此当前公式只消费已经实现的 HP, 等级, 敏捷, 运气和魅力.
func combatCaptureChance(source, target *combatUnitRuntimeState) float32 {
	hp, maxHP := float32(target.hp), float32(target.maxHP)
	if maxHP <= 0 {
		maxHP = 1
	}
	hpTerm := float32(10) - hp*hp/maxHP
	levelTerm := float32(source.unit.GetAttribute().GetLevel())/2 - float32(target.unit.GetAttribute().GetLevel())/2
	agilityTerm := float32(combatEffectiveAgilityPower(source))/15 - float32(combatEffectiveAgilityPower(target))/15
	base := float32(target.captureBase) + float32(combatEffectiveLuck(source))
	chance := (hpTerm + levelTerm + agilityTerm + base) * float32(source.charm) / 50
	if chance > 99 {
		return 99
	}
	return chance
}

// executeCapture 只生成 Capture 和成功后的 UnitLeave, 不造成伤害, 不触发反击或击杀收益.
func (r *CombatRoom) executeCapture(action *combatAction, events *[]*combatStepResult) {
	source := r.stateByKey(action.unitKey)
	if source == nil || !r.isAlive(action.unitKey) || !combatUnitIsPlayerCharacter(source.unit) {
		return
	}
	target := r.resolveCombatTarget(action)
	detail := &pb.CombatCaptureDetail{Result: pb.CombatCaptureResult_CombatCaptureResult_Failed}
	targetKey := cloneCombatUnitKey(action.targetKey)
	if target == nil {
		detail.FailureReason = pb.CombatCaptureFailureReason_CombatCaptureFailureReason_InvalidTarget
	} else {
		targetKey = cloneCombatUnitKey(target.unit.GetKey())
		action.targetKey = targetKey
		switch {
		case !target.pveEnemy || target.captureSnapshot == nil ||
			uint64(source.unit.GetAttribute().GetLevel())+5 < uint64(target.unit.GetAttribute().GetLevel()):
			detail.FailureReason = pb.CombatCaptureFailureReason_CombatCaptureFailureReason_NotCapturable
		case float32(r.random.rangeInt(1, 100)) >= combatCaptureChance(source, target):
			detail.FailureReason = pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Probability
		default:
			result := r.persistCapturedEnemy(source, target)
			if result.err != nil {
				xlog.GLog.Errorf("capture pet save failed battle:%s source:%s target:%s err:%v", r.battleID,
					combatUnitKeyMapKey(action.unitKey), combatUnitKeyMapKey(targetKey), result.err)
			}
			if result.petUUID == 0 {
				detail.FailureReason = result.reason
			} else {
				detail.Result = pb.CombatCaptureResult_CombatCaptureResult_Success
				detail.CapturedPetUuid = result.petUUID
				// escaped 是现有服务端统一离场标记; 协议用 Captured 区分捕获与逃跑, 不下发逃跑状态.
				target.escaped = true
				target.guard = false
				target.charge = nil
			}
		}
	}
	step := &combatStepResult{
		EventKind: combatStepKindAction, SkillId: action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{targetKey},
	}
	combatAppendEffect(step, &combatEffectResult{
		EffectKind: combatEffectKindCapture, Capture: detail,
		SourceUnitKeyList: cloneCombatUnitKeyList(step.SourceUnitKeyList),
		TargetUnitKeyList: cloneCombatUnitKeyList(step.TargetUnitKeyList),
	})
	if detail.GetResult() == pb.CombatCaptureResult_CombatCaptureResult_Success {
		combatAppendEffect(step, &combatEffectResult{
			EffectKind:        combatEffectKindUnitLeave,
			TargetUnitKeyList: cloneCombatUnitKeyList(step.TargetUnitKeyList),
			UnitLeaveReason:   pb.CombatUnitLeaveReason_CombatUnitLeaveReason_Captured,
		})
	}
	*events = append(*events, step)
}

func (r *CombatRoom) persistCapturedEnemy(source, target *combatUnitRuntimeState) combatRoomCaptureResult {
	key := source.unit.GetKey()
	participant := r.participant(combatRoomParticipantKey{aid: key.GetAid(), characterUUID: key.GetCharacterUuid()})
	if participant == nil || participant.account == nil || participant.account.actor == nil || r.actor == nil {
		return combatRoomCaptureResult{
			reason: pb.CombatCaptureFailureReason_CombatCaptureFailureReason_Persistence,
			err:    fmt.Errorf("capture pet participant account is unavailable"),
		}
	}
	return participant.account.PostCaptureCombatPetSync(combatRoomCaptureInput{
		characterUUID: key.GetCharacterUuid(), combatRoom: r.actor,
		gateway: participant.gateway, snapshot: target.captureSnapshot,
	})
}
