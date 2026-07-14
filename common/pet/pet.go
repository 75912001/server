package pet

import (
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
	for i := 0; i < rawRandomPointCount; i++ {
		// 每个随机点独立选择一项属性, 保证四项等概率且总点数固定.
		switch xutil.RandomU32(0, 3) {
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

// upgrade 使用加载宠物配置时生成的 Rank 计算逐级成长, 不在升级时重复推导 Rank.
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
		// 零次升级必须保留调用方传入的当前 Raw, 避免 1 级宠物的初始属性被清零.
		return rawVital, rawStr, rawTough, rawDex
	}
	rankMin, rankMax := rankGrowthRange(pet.Growth.Rank)
	for i := uint32(0); i < upgradeCount; i++ {
		randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
		const randomPrecision = uint64(1_000_000_000)
		randomRatio := float64(xutil.RandomU64(0, randomPrecision)) / float64(randomPrecision)
		rankRand := rankMin + randomRatio*(rankMax-rankMin)
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
		AssetId:            uint64(*pet.ID),
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

func rankGrowthRange(rank uint32) (float64, float64) {
	switch rank {
	case 0:
		return 4.50, 5.00
	case 1:
		return 4.70, 5.20
	case 2:
		return 4.90, 5.40
	case 3:
		return 5.10, 5.60
	case 4:
		return 5.30, 5.80
	case 5:
		return 5.50, 6.00
	default:
		panic("invalid pet growth rank")
	}
}
