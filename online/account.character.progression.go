package main

import (
	"fmt"
	"math"

	"server/common/gameconfig"
	commonpet "server/common/pet"
	pb "server/proto/pb"
)

const (
	characterAvailablePointPerLevel   uint32 = 3
	characterCharmPerLevel            uint32 = 2
	characterInitialDuelPoint         uint32 = 100
	characterInitialCharm             uint32 = 60
	characterMaxCharm                 uint32 = 100
	characterMaxReputation            uint32 = 100_000_000
	characterReputationExpDivisor     uint64 = 20_000
	petOwnerReputationMinimumNewLevel uint32 = 31
)

type experienceSettlement struct {
	PreviousExp     uint64
	CurrentExp      uint64
	AppliedExp      uint64
	OldLevel        uint32
	NewLevel        uint32
	ReputationDelta uint32
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

// characterLevelUpReputation 按原版人物公式逐级计算声望.
// 当前累计经验表的等级最小经验等于原版 LevelUpTbl 的同级值.
func characterLevelUpReputation(oldLevel uint32, newLevel uint32) (uint64, error) {
	reputation := uint64(0)
	for level := oldLevel + 1; level <= newLevel; level++ {
		levelExp, err := gameconfig.GGameConfig.Exp.GetLevelMinExp(level)
		if err != nil {
			return 0, fmt.Errorf("get character reputation level %d experience: %w", level, err)
		}
		reputation += levelExp / characterReputationExpDivisor
	}
	return reputation, nil
}

// petOwnerLevelUpReputation 按原版宠物公式逐级计算主人声望.
// 宠物到达新等级 L 时使用 LevelUpTbl[L-1], 且只有 L 大于 30 才结算.
func petOwnerLevelUpReputation(oldLevel uint32, newLevel uint32) (uint64, error) {
	firstLevel := oldLevel + 1
	if firstLevel < petOwnerReputationMinimumNewLevel {
		firstLevel = petOwnerReputationMinimumNewLevel
	}
	reputation := uint64(0)
	for level := firstLevel; level <= newLevel; level++ {
		levelExp, err := gameconfig.GGameConfig.Exp.GetLevelMinExp(level - 1)
		if err != nil {
			return 0, fmt.Errorf("get pet owner reputation level %d experience: %w", level, err)
		}
		reputation += levelExp / characterReputationExpDivisor
	}
	return reputation, nil
}

func addCharacterReputation(base *pb.CharacterBaseRecord, added uint64) uint32 {
	current := uint64(base.GetReputation())
	remaining := uint64(characterMaxReputation) - current
	if added > remaining {
		added = remaining
	}
	base.Reputation = uint32(current + added)
	return uint32(added)
}

// applyCharacterExperience 统一结算角色EXP, 可分配点, DP, 魅力和声望.
func applyCharacterExperience(record *pb.CharacterRecord, addedExp uint64) (experienceSettlement, error) {
	if record == nil || record.GetBase() == nil {
		return experienceSettlement{}, fmt.Errorf("character record is nil")
	}
	base := record.GetBase()
	if base.GetReputation() > characterMaxReputation {
		return experienceSettlement{}, fmt.Errorf("character reputation %d exceeds limit", base.GetReputation())
	}
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
	reputation, err := characterLevelUpReputation(settlement.OldLevel, settlement.NewLevel)
	if err != nil {
		return experienceSettlement{}, err
	}

	base.Exp = settlement.CurrentExp
	base.AvailablePoint += uint32(availablePointDelta)
	base.DuelPoint = uint32(duelPoint)
	base.Charm = uint32(charm)
	settlement.ReputationDelta = addCharacterReputation(base, reputation)
	return settlement, nil
}

// applyPetExperience 复用同一EXP等级表, 并按实际提升等级数结算宠物成长和所属角色声望.
func applyPetExperience(record *pb.PetRecord, ownerBase *pb.CharacterBaseRecord, addedExp uint64) (experienceSettlement, error) {
	if record == nil || ownerBase == nil {
		return experienceSettlement{}, fmt.Errorf("pet record or owner base is nil")
	}
	if ownerBase.GetReputation() > characterMaxReputation {
		return experienceSettlement{}, fmt.Errorf("pet owner reputation %d exceeds limit", ownerBase.GetReputation())
	}
	settlement, err := calculateExperienceSettlement(record.GetExp(), addedExp)
	if err != nil {
		return experienceSettlement{}, err
	}
	reputation, err := petOwnerLevelUpReputation(settlement.OldLevel, settlement.NewLevel)
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
	settlement.ReputationDelta = addCharacterReputation(ownerBase, reputation)
	return settlement, nil
}
