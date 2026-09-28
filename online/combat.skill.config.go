package main

import (
	"fmt"

	"server/common/gameconfig"
)

// cloneEnemyBattleAI复制已校验AI的技能、权重及目标策略, 本场战斗不受配置修改影响.
func cloneEnemyBattleAI(ai *gameconfig.BattleAIEntry) *gameconfig.BattleAIEntry {
	if ai == nil {
		return nil
	}
	snapshot := *ai
	if ai.ID != nil {
		id := *ai.ID
		snapshot.ID = &id
	}
	snapshot.Skills = make([]gameconfig.BattleAISkillEntry, len(ai.Skills))
	for index, skill := range ai.Skills {
		id, weight := *skill.ID, *skill.Weight
		snapshot.Skills[index] = gameconfig.BattleAISkillEntry{ID: &id, Weight: &weight}
	}
	scope, selection := *ai.TargetScope, *ai.TargetSelection
	snapshot.TargetScope, snapshot.TargetSelection = &scope, &selection
	if ai.TargetRandomRollMax != nil {
		rollMax := *ai.TargetRandomRollMax
		snapshot.TargetRandomRollMax = &rollMax
	}
	return &snapshot
}

// validateEnemyCombatSkillConfig在Online产生任何外部副作用前校验NPC执行能力.
// 通用配置层负责结构、引用和AI权重, 这里负责拒绝online尚未实现的技能行为.
func validateEnemyCombatSkillConfig() error {
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Enemy == nil ||
		gameconfig.GGameConfig.Pet == nil || gameconfig.GGameConfig.Character == nil ||
		gameconfig.GGameConfig.Skill == nil {
		return fmt.Errorf("enemy, appearance or skill config is not loaded")
	}

	var validationErr error
	gameconfig.GGameConfig.Enemy.Foreach(func(groupID uint32, group *gameconfig.EnemyGroupEntry) bool {
		if group == nil {
			validationErr = fmt.Errorf("enemy group is nil: group:%d", groupID)
			return false
		}
		for enemyIndex := range group.Enemies {
			enemy := &group.Enemies[enemyIndex]
			assetID := enemy.AssetID()
			if assetID == 0 {
				validationErr = fmt.Errorf("enemy appearance id is nil: group:%d index:%d", groupID, enemyIndex)
				return false
			}
			if enemy.IsPet() && gameconfig.GGameConfig.Pet.Get(assetID) == nil {
				validationErr = fmt.Errorf("enemy pet config is missing: group:%d pet:%d", groupID, assetID)
				return false
			}
			if enemy.IsCharacter() && gameconfig.GGameConfig.Character.Get(assetID) == nil {
				validationErr = fmt.Errorf("enemy character config is missing: group:%d character:%d", groupID, assetID)
				return false
			}
			if enemy.BattleAI == nil {
				validationErr = fmt.Errorf("enemy AI config is missing: group:%d enemy:%d", groupID, assetID)
				return false
			}
			for _, action := range enemy.BattleAI.Skills {
				skillID := *action.ID
				skill := gameconfig.GGameConfig.Skill.Get(skillID)
				if skill == nil {
					validationErr = fmt.Errorf("enemy skill config is missing: group:%d enemy:%d skill:%d",
						groupID, assetID, skillID)
					return false
				}
				if !enemyCombatSkillSupported(skillID, skill) {
					validationErr = fmt.Errorf("enemy skill is not supported by online: group:%d enemy:%d skill:%d",
						groupID, assetID, skillID)
					return false
				}
			}
		}
		return true
	})
	return validationErr
}

func enemyCombatSkillSupported(skillID uint32, skill *gameconfig.SkillEntry) bool {
	if skill == nil {
		return false
	}
	switch skillID {
	case combatSkillAttack, combatSkillDefense, combatSkillEscape, combatSkillStandby, combatSkillGuardBreak:
		return true
	default:
		return (skill.ContinuationAttack != nil && skill.ContinuationAttack.SegmentCount != nil) ||
			(skill.MightyAttack != nil && skill.MightyAttack.DamageMultiplier != nil && skill.MightyAttack.TargetDodgeBonus != nil) ||
			(skill.PoisonAttack != nil && skill.PoisonAttack.DurationActions != nil && skill.PoisonAttack.AttackPercentModifier != nil) ||
			(skill.StoneAttack != nil && skill.StoneAttack.DurationActions != nil && skill.StoneAttack.AttackPercentModifier != nil) ||
			(skill.ConfusionAttack != nil && skill.ConfusionAttack.DurationActions != nil && skill.ConfusionAttack.AttackPercentModifier != nil) ||
			(skill.DrunkAttack != nil && skill.DrunkAttack.DurationActions != nil && skill.DrunkAttack.AttackPercentModifier != nil) ||
			(skill.SleepAttack != nil && skill.SleepAttack.DurationActions != nil && skill.SleepAttack.AttackPercentModifier != nil) ||
			(skill.ChargeAttack != nil && skill.ChargeAttack.ChargeRounds != nil && skill.ChargeAttack.AttackPercentModifier != nil) ||
			(skill.EarthRound != nil && skill.EarthRound.DamagePercentModifier != nil) ||
			(skill.Guardian != nil && skill.Guardian.AttackPercentModifier != nil) ||
			(skill.NoGuard != nil && skill.NoGuard.DodgePercent != nil && skill.NoGuard.CounterPercent != nil && skill.NoGuard.CriticalPercent != nil) ||
			(skill.PowerBalance != nil && skill.PowerBalance.AttackPercentModifier != nil && skill.PowerBalance.DefensePercentModifier != nil) ||
			skill.ShowMercy != nil || skill.Abduct != nil
	}
}
