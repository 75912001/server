package pet

import (
	"math/rand"
	"server/common/gameconfig"
	"server/proto/pb"
	"time"

	xutil "github.com/75912001/xlib/util"
)

const (
	rawRandomPointCount   = 10 // 随机点数数量
	petSavedBaseOffsetMin = -2 // 宠物偏移量-最小值
	petSavedBaseOffsetMax = 2  // 宠物偏移量-最大值
)

// CalculateHP 计算宠物-hp
func CalculateHP(rawVital uint32, rawStr uint32, rawTough uint32, rawDex uint32) uint32 {
	return uint32((float64(rawVital)*4.0 + float64(rawStr) + float64(rawTough) + float64(rawDex)) * 0.01)
}

// CalculateAttack 计算宠物-攻击
func CalculateAttack(rawVital uint32, rawStr uint32, rawTough uint32, rawDex uint32) uint32 {
	return uint32(float64(rawStr)*0.01 + float64(rawTough)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

// CalculateDefense 计算宠物-防御
func CalculateDefense(rawVital uint32, rawStr uint32, rawTough uint32, rawDex uint32) uint32 {
	return uint32(float64(rawTough)*0.01 + float64(rawStr)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

// CalculateAgility 计算宠物-敏捷
func CalculateAgility(rawDex uint32) uint32 {
	return uint32(float64(rawDex) * 0.01)
}

// 随机-4属性-分布
func randomFourPointDistribution() (vital uint32, str uint32, tough uint32, dex uint32) {
	randomU32 := xutil.RandomU32(0, 4)
	for i := 0; i < rawRandomPointCount; i++ {
		switch randomU32 {
		case 0:
			vital++
		case 1:
			str++
		case 2:
			tough++
		default:
			dex++
		}
	}
	return vital, str, tough, dex
}

// 升级-宠物
func upgrade(pet *gameconfig.PetEntry, upgradeCount uint32,
	savedBaseVital uint32,
	savedBaseStr uint32,
	savedBaseTough uint32,
	savedBaseDex uint32,
	rawVital uint32,
	rawStr uint32,
	rawTough uint32,
	rawDex uint32) (newRawVital uint32,
	newRawStr uint32,
	newRawTough uint32,
	newRawDex uint32) {
	if upgradeCount == 0 {
		return
	}
	baseSum := int(*pet.Growth.BaseVital) + int(*pet.Growth.BaseStr) + int(*pet.Growth.BaseTough) + int(*pet.Growth.BaseDex)
	rankMin, rankMax := rankGrowthRange(baseSum)
	for i := uint32(0); i < upgradeCount; i++ {
		randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
		rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(i)))
		rankRand := rankMin + rng.Float64()*(rankMax-rankMin)
		rawVital += uint32(float64(savedBaseVital+randomVital) * rankRand)
		rawStr += uint32(float64(savedBaseStr+randomStr) * rankRand)
		rawTough += uint32(float64(savedBaseTough+randomTough) * rankRand)
		rawDex += uint32(float64(savedBaseDex+randomDex) * rankRand)
	}
	return rawVital, rawStr, rawTough, rawDex
}

// 创建
func create(pet *gameconfig.PetEntry, level uint32, grade pb.PetGrade) (
	savedBaseVital uint32,
	savedBaseStr uint32,
	savedBaseTough uint32,
	savedBaseDex uint32,
	rawVital uint32,
	rawStr uint32,
	rawTough uint32,
	rawDex uint32) {
	var gradeOffset int
	switch grade {
	case pb.PetGrade_PetGrade_Common:
		gradeOffset = petSavedBaseOffsetMin
	case pb.PetGrade_PetGrade_Rare:
		gradeOffset = -1
	case pb.PetGrade_PetGrade_Epic:
		gradeOffset = 0
	case pb.PetGrade_PetGrade_Legendary:
		gradeOffset = 1
	case pb.PetGrade_PetGrade_Mythic:
		gradeOffset = petSavedBaseOffsetMax
	}

	savedBaseVital = uint32(int(*pet.Growth.BaseVital) + gradeOffset)
	savedBaseStr = uint32(int(*pet.Growth.BaseStr) + gradeOffset)
	savedBaseTough = uint32(int(*pet.Growth.BaseTough) + gradeOffset)
	savedBaseDex = uint32(int(*pet.Growth.BaseDex) + gradeOffset)

	randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
	initialFactor := float64(*pet.Growth.InitNum)

	rawVital = uint32(initialFactor * float64(savedBaseVital+randomVital))
	rawStr = uint32(initialFactor * float64(savedBaseStr+randomStr))
	rawTough = uint32(initialFactor * float64(savedBaseTough+randomTough))
	rawDex = uint32(initialFactor * float64(savedBaseDex+randomDex))

	rawVital, rawStr, rawTough, rawDex = upgrade(pet, level-1, savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex)

	return savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex
}

// NewRecord 创建-宠物
func NewRecord(pet *gameconfig.PetEntry, petUUID uint64, level uint32, grade pb.PetGrade) *pb.PetRecord {
	expMin, _ := gameconfig.GGameConfig.Exp.GetLevelMinExp(level)
	savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex := create(pet, level, grade)
	return &pb.PetRecord{
		Uuid:               petUUID,
		CarryStatus:        pb.PetCarryStatus_PetCarryStatus_Rest,
		Grade:              grade,
		Exp:                expMin,
		Loyalty:            100,
		SavedBaseVitality:  savedBaseVital,
		SavedBaseStrength:  savedBaseStr,
		SavedBaseToughness: savedBaseTough,
		SavedBaseDexterity: savedBaseDex,
		RawVitality:        rawVital,
		RawStrength:        rawStr,
		RawToughness:       rawTough,
		RawDexterity:       rawDex,
		CreateTimestampMs:  time.Now().UnixMilli(),
		AssetRecordBaseMap: make(map[uint32]uint64),
		RecordMap:          make(map[uint64]*pb.RecordPrimary),
	}
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
