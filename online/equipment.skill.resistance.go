package main

import (
	"server/common/gameconfig"
	pb "server/proto/pb"
)

const equipmentStatusSpiritResistanceBonus int64 = 10

var equipmentStatusSpiritResistanceAttributes = [...]pb.EquipmentRecordAttribute{
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_PoisonResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance,
}

// equipmentStatusSpiritResistanceAttribute识别附加技能所属的五系异常精灵类别.
// 配置自带技能不经过本函数, 继续完全使用武器配置中实例化的基础抗性.
func equipmentStatusSpiritResistanceAttribute(skillID uint32) (pb.EquipmentRecordAttribute, bool) {
	if skillID == 0 || gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Skill == nil {
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Unspecified, false
	}
	skill := gameconfig.GGameConfig.Skill.Get(skillID)
	if skill == nil {
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Unspecified, false
	}
	switch {
	case skill.PoisonSpirit != nil:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_PoisonResistance, true
	case skill.SleepSpirit != nil:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance, true
	case skill.StoneSpirit != nil:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance, true
	case skill.DrunkSpirit != nil:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance, true
	case skill.ConfusionSpirit != nil:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance, true
	default:
		return pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Unspecified, false
	}
}

// equipmentSkillResistanceModifiers根据实例附加技能实时派生五系抗性修正.
// 已拥有类别固定为+10, 未拥有类别按不同精灵类别数每类-10, 麻痹不参与.
func equipmentSkillResistanceModifiers(record *pb.EquipmentRecord) map[pb.EquipmentRecordAttribute]int64 {
	active := make(map[pb.EquipmentRecordAttribute]struct{}, len(equipmentStatusSpiritResistanceAttributes))
	if record != nil {
		for _, skillID := range record.GetAdditionalSkillIdList() {
			if attribute, ok := equipmentStatusSpiritResistanceAttribute(skillID); ok {
				active[attribute] = struct{}{}
			}
		}
	}
	if len(active) == 0 {
		return nil
	}
	modifiers := make(map[pb.EquipmentRecordAttribute]int64, len(equipmentStatusSpiritResistanceAttributes))
	penalty := -equipmentStatusSpiritResistanceBonus * int64(len(active))
	for _, attribute := range equipmentStatusSpiritResistanceAttributes {
		if _, exists := active[attribute]; exists {
			modifiers[attribute] = equipmentStatusSpiritResistanceBonus
		} else {
			modifiers[attribute] = penalty
		}
	}
	return modifiers
}

func equipmentSkillResistanceModifier(record *pb.EquipmentRecord, key pb.EquipmentRecordAttribute) int64 {
	switch key {
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_PoisonResistance,
		pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance,
		pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance,
		pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance,
		pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance:
	default:
		return 0
	}
	return equipmentSkillResistanceModifiers(record)[key]
}
