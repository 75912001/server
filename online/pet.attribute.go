package main

import (
	"fmt"
	"math/rand"

	"server/common/gameconfig"
	pb "server/proto/pb"
)

const (
	rawRandomPointCount   = 10
	petSavedBaseOffsetMin = -2
	petSavedBaseOffsetMax = 2
)

type petRuntimeAttribute struct {
	savedBaseVital uint64
	savedBaseStr   uint64
	savedBaseTough uint64
	savedBaseDex   uint64
	rawVital       uint64
	rawStr         uint64
	rawTough       uint64
	rawDex         uint64
	hp             uint64
	attack         uint64
	defense        uint64
	agility        uint64
}

func createPetRuntimeAttribute(petID uint32, level int, grade pb.PetGrade, rng *rand.Rand) (*petRuntimeAttribute, error) {
	if GGameConfig == nil || GGameConfig.Pet == nil {
		return nil, fmt.Errorf("game config is not loaded")
	}
	if level <= 0 {
		return nil, fmt.Errorf("pet level invalid: %d", level)
	}
	pet := GGameConfig.Pet.GetByID(int(petID))
	if pet == nil {
		return nil, fmt.Errorf("pet config not found: %d", petID)
	}
	if rng == nil {
		return nil, fmt.Errorf("rng is nil")
	}
	gradeOffset, err := petGradeSavedBaseOffset(grade)
	if err != nil {
		return nil, err
	}

	attr := &petRuntimeAttribute{
		savedBaseVital: uint64(pet.Growth.BaseVital + gradeOffset),
		savedBaseStr:   uint64(pet.Growth.BaseStr + gradeOffset),
		savedBaseTough: uint64(pet.Growth.BaseTough + gradeOffset),
		savedBaseDex:   uint64(pet.Growth.BaseDex + gradeOffset),
	}
	initialBonus := randomFourPointDistribution(rng)
	initialFactor := float64(pet.Growth.InitNum)
	attr.rawVital = uint64(initialFactor * float64(int(attr.savedBaseVital)+initialBonus.vital))
	attr.rawStr = uint64(initialFactor * float64(int(attr.savedBaseStr)+initialBonus.str))
	attr.rawTough = uint64(initialFactor * float64(int(attr.savedBaseTough)+initialBonus.tough))
	attr.rawDex = uint64(initialFactor * float64(int(attr.savedBaseDex)+initialBonus.dex))

	upgradePetRuntimeAttribute(pet, level-1, attr, rng)
	attr.hp = calculatePetHP(attr.rawVital, attr.rawStr, attr.rawTough, attr.rawDex)
	attr.attack = calculatePetAttack(attr.rawVital, attr.rawStr, attr.rawTough, attr.rawDex)
	attr.defense = calculatePetDefense(attr.rawVital, attr.rawStr, attr.rawTough, attr.rawDex)
	attr.agility = calculatePetAgility(attr.rawDex)
	return attr, nil
}

func petGradeSavedBaseOffset(grade pb.PetGrade) (int, error) {
	switch grade {
	case pb.PetGrade_PetGrade_Common:
		return petSavedBaseOffsetMin, nil
	case pb.PetGrade_PetGrade_Rare:
		return -1, nil
	case pb.PetGrade_PetGrade_Epic:
		return 0, nil
	case pb.PetGrade_PetGrade_Legendary:
		return 1, nil
	case pb.PetGrade_PetGrade_Mythic:
		return petSavedBaseOffsetMax, nil
	default:
		return 0, fmt.Errorf("pet grade invalid: %s", grade.String())
	}
}

func petRuntimeAttributeFromPetRecord(pet *pb.PetRecord) *petRuntimeAttribute {
	if pet == nil {
		return nil
	}
	rawVital := pet.GetRawVitality()
	rawStr := pet.GetRawStrength()
	rawTough := pet.GetRawToughness()
	rawDex := pet.GetRawDexterity()
	if rawVital == 0 || rawStr == 0 || rawTough == 0 || rawDex == 0 {
		return nil
	}
	return &petRuntimeAttribute{
		rawVital: rawVital,
		rawStr:   rawStr,
		rawTough: rawTough,
		rawDex:   rawDex,
		hp:       calculatePetHP(rawVital, rawStr, rawTough, rawDex),
		attack:   calculatePetAttack(rawVital, rawStr, rawTough, rawDex),
		defense:  calculatePetDefense(rawVital, rawStr, rawTough, rawDex),
		agility:  calculatePetAgility(rawDex),
	}
}

func upgradePetRuntimeAttribute(pet *gameconfig.PetEntry, upgradeCount int, attr *petRuntimeAttribute, rng *rand.Rand) {
	if upgradeCount <= 0 || pet == nil || attr == nil || rng == nil {
		return
	}
	rankMin, rankMax := rankGrowthRange(pet.Growth.BaseVital + pet.Growth.BaseStr + pet.Growth.BaseTough + pet.Growth.BaseDex)
	for i := 0; i < upgradeCount; i++ {
		addPoints := randomFourPointDistribution(rng)
		rankRand := rankMin + rng.Float64()*(rankMax-rankMin)
		attr.rawVital += uint64(float64(int(attr.savedBaseVital)+addPoints.vital) * rankRand)
		attr.rawStr += uint64(float64(int(attr.savedBaseStr)+addPoints.str) * rankRand)
		attr.rawTough += uint64(float64(int(attr.savedBaseTough)+addPoints.tough) * rankRand)
		attr.rawDex += uint64(float64(int(attr.savedBaseDex)+addPoints.dex) * rankRand)
	}
}

type fourPointDistribution struct {
	vital int
	str   int
	tough int
	dex   int
}

func randomFourPointDistribution(rng *rand.Rand) fourPointDistribution {
	var out fourPointDistribution
	for i := 0; i < rawRandomPointCount; i++ {
		switch rng.Intn(4) {
		case 0:
			out.vital++
		case 1:
			out.str++
		case 2:
			out.tough++
		default:
			out.dex++
		}
	}
	return out
}

func rankGrowthRange(baseSum int) (float64, float64) {
	if baseSum >= 100 {
		return 4.50, 5.00
	}
	if baseSum >= 95 {
		return 4.70, 5.20
	}
	if baseSum >= 90 {
		return 4.90, 5.40
	}
	if baseSum >= 85 {
		return 5.10, 5.60
	}
	if baseSum >= 80 {
		return 5.30, 5.80
	}
	return 5.50, 6.00
}

func calculatePetHP(rawVital uint64, rawStr uint64, rawTough uint64, rawDex uint64) uint64 {
	return uint64((float64(rawVital)*4.0 + float64(rawStr) + float64(rawTough) + float64(rawDex)) * 0.01)
}

func calculatePetAttack(rawVital uint64, rawStr uint64, rawTough uint64, rawDex uint64) uint64 {
	return uint64(float64(rawStr)*0.01 + float64(rawTough)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

func calculatePetDefense(rawVital uint64, rawStr uint64, rawTough uint64, rawDex uint64) uint64 {
	return uint64(float64(rawTough)*0.01 + float64(rawStr)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

func calculatePetAgility(rawDex uint64) uint64 {
	return uint64(float64(rawDex) * 0.01)
}
