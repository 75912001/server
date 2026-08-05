package main

import (
	"fmt"
	"math"

	"server/common/gameconfig"
	commonpet "server/common/pet"
	pb "server/proto/pb"
)

const (
	characterAvailablePointPerLevel uint32 = 3
	characterCharmPerLevel          uint32 = 2
	characterInitialDuelPoint       uint32 = 100
	characterInitialCharm           uint32 = 60
	characterMaxCharm               uint32 = 100
)

type experienceSettlement struct {
	PreviousExp uint64
	CurrentExp  uint64
	AppliedExp  uint64
	OldLevel    uint32
	NewLevel    uint32
}

func (p experienceSettlement) levelUpCount() uint32 {
	if p.NewLevel <= p.OldLevel {
		return 0
	}
	return p.NewLevel - p.OldLevel
}

// calculateExperienceSettlement 是所有角色和宠物经验来源的统一等级门槛入口.
// 它先完成纯计算和边界校验, 再由目标结算函数写入各自的升级副作用.
func calculateExperienceSettlement(currentExp uint64, addedExp uint64) (experienceSettlement, error) {
	if addedExp == 0 {
		return experienceSettlement{}, fmt.Errorf("added experience is zero")
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Exp == nil {
		return experienceSettlement{}, fmt.Errorf("experience config is not loaded")
	}
	oldLevel, err := gameconfig.GGameConfig.Exp.GetLevel(currentExp)
	if err != nil {
		return experienceSettlement{}, err
	}
	maxTotalExp, err := gameconfig.GGameConfig.Exp.GetMaxTotalExp()
	if err != nil {
		return experienceSettlement{}, err
	}
	if currentExp > maxTotalExp {
		return experienceSettlement{}, fmt.Errorf("current experience %d exceeds maximum %d", currentExp, maxTotalExp)
	}
	appliedExp := addedExp
	if remaining := maxTotalExp - currentExp; appliedExp > remaining {
		appliedExp = remaining
	}
	currentExp += appliedExp
	newLevel, err := gameconfig.GGameConfig.Exp.GetLevel(currentExp)
	if err != nil {
		return experienceSettlement{}, err
	}
	return experienceSettlement{
		PreviousExp: currentExp - appliedExp,
		CurrentExp:  currentExp,
		AppliedExp:  appliedExp,
		OldLevel:    oldLevel,
		NewLevel:    newLevel,
	}, nil
}

// applyCharacterExperience 统一结算角色EXP、可分配点、DP和魅力.
func applyCharacterExperience(record *pb.CharacterRecord, addedExp uint64) (experienceSettlement, error) {
	if record == nil || record.GetBase() == nil {
		return experienceSettlement{}, fmt.Errorf("character record is nil")
	}
	base := record.GetBase()
	settlement, err := calculateExperienceSettlement(base.GetExp(), addedExp)
	if err != nil {
		return experienceSettlement{}, err
	}
	levelUpCount := settlement.levelUpCount()
	availablePointDelta := uint64(levelUpCount) * uint64(characterAvailablePointPerLevel)
	if uint64(base.GetAvailablePoint())+availablePointDelta > math.MaxUint32 {
		return experienceSettlement{}, fmt.Errorf("character available point overflows uint32")
	}
	duelPoint := uint64(base.GetDuelPoint())
	for level := settlement.OldLevel + 1; level <= settlement.NewLevel; level++ {
		duelPoint += uint64(level) * 10
	}
	if duelPoint > uint64(pb.CharacterLimit_CharacterLimit_MaxDuelPoint) {
		duelPoint = uint64(pb.CharacterLimit_CharacterLimit_MaxDuelPoint)
	}
	charm := uint64(base.GetCharm()) + uint64(levelUpCount)*uint64(characterCharmPerLevel)
	if charm > uint64(characterMaxCharm) {
		charm = uint64(characterMaxCharm)
	}

	base.Exp = settlement.CurrentExp
	base.AvailablePoint += uint32(availablePointDelta)
	base.DuelPoint = uint32(duelPoint)
	base.Charm = uint32(charm)
	return settlement, nil
}

// applyPetExperience 复用同一EXP等级表, 并把宠物专属成长按实际提升等级数完整结算.
func applyPetExperience(record *pb.PetRecord, addedExp uint64) (experienceSettlement, error) {
	if record == nil {
		return experienceSettlement{}, fmt.Errorf("pet record is nil")
	}
	settlement, err := calculateExperienceSettlement(record.GetExp(), addedExp)
	if err != nil {
		return experienceSettlement{}, err
	}
	if err := commonpet.EnsureGrowthBaseline(record, settlement.OldLevel); err != nil {
		return experienceSettlement{}, err
	}
	if err := commonpet.UpgradeRecord(record, settlement.levelUpCount()); err != nil {
		return experienceSettlement{}, err
	}
	record.Exp = settlement.CurrentExp
	return settlement, nil
}
