package pet

import (
	"fmt"
	"server/common/gameconfig"
	"server/proto/pb"
	"time"

	xutil "github.com/75912001/xlib/util"
)

// UpgradeRecord 按实际提升等级数结算宠物原始属性成长.
// 经验和等级门槛由调用方统一结算, 本函数只负责宠物专属的逐级成长副作用.
func UpgradeRecord(record *pb.PetRecord, upgradeCount uint32) error {
	if record == nil || record.GetAssetId() == 0 {
		return fmt.Errorf("pet record or asset id is empty")
	}
	if upgradeCount == 0 {
		return nil
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Pet == nil {
		return fmt.Errorf("pet config is not loaded: %d", record.GetAssetId())
	}
	petEntry := gameconfig.GGameConfig.Pet.Get(record.GetAssetId())
	if petEntry == nil {
		return fmt.Errorf("pet config not found: %d", record.GetAssetId())
	}
	record.RawVitality, record.RawStrength, record.RawToughness, record.RawDexterity = upgrade(
		petEntry,
		upgradeCount,
		record.GetSavedBaseVitality(),
		record.GetSavedBaseStrength(),
		record.GetSavedBaseToughness(),
		record.GetSavedBaseDexterity(),
		record.GetRawVitality(),
		record.GetRawStrength(),
		record.GetRawToughness(),
		record.GetRawDexterity(),
	)
	return nil
}

const (
	rawRandomPointCount   = 10 // 随机点数数量
	petSavedBaseOffsetMin = -2 // 宠物偏移量-最小值
	petSavedBaseOffsetMax = 2  // 宠物偏移量-最大值
)

// EnsureGrowthBaseline 按宠物当前等级维护成长率基准.
// 当前等级为 1 时强制覆盖; 高等级仅在基准缺失时记录当前等级和派生属性.
func EnsureGrowthBaseline(record *pb.PetRecord, currentLevel uint32) error {
	if record == nil {
		return fmt.Errorf("pet record is nil")
	}
	if currentLevel < uint32(pb.LevelRange_LevelRange_Min) || currentLevel > uint32(pb.LevelRange_LevelRange_Max) {
		return fmt.Errorf("pet level is out of range: %d", currentLevel)
	}
	if record.GetRawVitality() == 0 || record.GetRawStrength() == 0 || record.GetRawToughness() == 0 || record.GetRawDexterity() == 0 {
		return fmt.Errorf("pet raw attribute is incomplete")
	}
	if currentLevel == uint32(pb.LevelRange_LevelRange_Min) {
		recordGrowthBaseline(record, currentLevel)
		return nil
	}
	if baselineLevel := record.GetGrowthBaselineLevel(); baselineLevel != 0 {
		if baselineLevel > currentLevel || baselineLevel > uint32(pb.LevelRange_LevelRange_Max) {
			return fmt.Errorf("pet growth baseline level %d exceeds current level %d", baselineLevel, currentLevel)
		}
		return nil
	}
	if hasGrowthBaselineAttributes(record) {
		// 兼容字段改名前已经写入的真实 1 级快照; 字段编号未变化, 只需补齐来源等级.
		record.GrowthBaselineLevel = uint32(pb.LevelRange_LevelRange_Min)
		return nil
	}
	recordGrowthBaseline(record, currentLevel)
	return nil
}

func hasGrowthBaselineAttributes(record *pb.PetRecord) bool {
	return record.GetGrowthBaselineHp() != 0 ||
		record.GetGrowthBaselineAttack() != 0 ||
		record.GetGrowthBaselineDefense() != 0 ||
		record.GetGrowthBaselineAgility() != 0
}

func recordGrowthBaseline(record *pb.PetRecord, level uint32) {
	record.GrowthBaselineHp = gameconfig.CalculatePetPanelHP(record.GetRawVitality(), record.GetRawStrength(), record.GetRawToughness(), record.GetRawDexterity())
	record.GrowthBaselineAttack = gameconfig.CalculatePetPanelAttack(record.GetRawVitality(), record.GetRawStrength(), record.GetRawToughness(), record.GetRawDexterity())
	record.GrowthBaselineDefense = gameconfig.CalculatePetPanelDefense(record.GetRawVitality(), record.GetRawStrength(), record.GetRawToughness(), record.GetRawDexterity())
	record.GrowthBaselineAgility = gameconfig.CalculatePetPanelAgility(record.GetRawDexterity())
	record.GrowthBaselineLevel = level
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
	rankMin, rankMax := gameconfig.PetRankGrowthRange(pet.Growth.Rank)
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

// | 总偏移 | 组合数 | 概率 |
// |---:|---:|---:|
// | -8 | 1 | 0.16% |
// | -7 | 4 | 0.64% |
// | -6 | 10 | 1.60% |
// | -5 | 20 | 3.20% |
// | -4 | 35 | 5.60% |
// | -3 | 52 | 8.32% |
// | -2 | 68 | 10.88% |
// | -1 | 80 | 12.80% |
// | 0 | 85 | 13.60% |
// | 1 | 80 | 12.80% |
// | 2 | 68 | 10.88% |
// | 3 | 52 | 8.32% |
// | 4 | 35 | 5.60% |
// | 5 | 20 | 3.20% |
// | 6 | 10 | 1.60% |
// | 7 | 4 | 0.64% |
// | 8 | 1 | 0.16% |

// | 品阶 | 四维总偏移 | 组合数 | 概率 |
// |---|---:|---:|---:|
// | Common | `-8 ~ 0` | 355 | 56.80% |
// | Rare | `1 ~ 2` | 148 | 23.68% |
// | Epic | `3 ~ 4` | 87 | 13.92% |
// | Legendary | `5 ~ 6` | 30 | 4.80% |
// | Mythic | `7 ~ 8` | 5 | 0.80% |

func petGradeFromRandomOffsetTotal(totalOffset int) pb.PetGrade {
	switch {
	case totalOffset <= 0:
		return pb.PetGrade_PetGrade_Common
	case totalOffset <= 2:
		return pb.PetGrade_PetGrade_Rare
	case totalOffset <= 4:
		return pb.PetGrade_PetGrade_Epic
	case totalOffset <= 6:
		return pb.PetGrade_PetGrade_Legendary
	default:
		return pb.PetGrade_PetGrade_Mythic
	}
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
	rawDex uint32,
	actualGrade pb.PetGrade) {
	var vitalOffset, strOffset, toughOffset, dexOffset int
	if grade == pb.PetGrade_PetGrade_Unknow {
		// 未指定品阶时四维独立随机, 再按四维总偏移计算并保存实际品阶.
		const randomOffsetRange = uint32(petSavedBaseOffsetMax - petSavedBaseOffsetMin)
		vitalOffset = int(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		strOffset = int(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		toughOffset = int(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		dexOffset = int(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		grade = petGradeFromRandomOffsetTotal(vitalOffset + strOffset + toughOffset + dexOffset)
	} else {
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
		vitalOffset = gradeOffset
		strOffset = gradeOffset
		toughOffset = gradeOffset
		dexOffset = gradeOffset
	}

	savedBaseVital = uint32(int(*pet.Growth.BaseVital) + vitalOffset)
	savedBaseStr = uint32(int(*pet.Growth.BaseStr) + strOffset)
	savedBaseTough = uint32(int(*pet.Growth.BaseTough) + toughOffset)
	savedBaseDex = uint32(int(*pet.Growth.BaseDex) + dexOffset)

	randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
	initialFactor := float64(*pet.Growth.InitNum)

	rawVital = uint32(initialFactor * float64(savedBaseVital+randomVital))
	rawStr = uint32(initialFactor * float64(savedBaseStr+randomStr))
	rawTough = uint32(initialFactor * float64(savedBaseTough+randomTough))
	rawDex = uint32(initialFactor * float64(savedBaseDex+randomDex))

	rawVital, rawStr, rawTough, rawDex = upgrade(pet, level-1, savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex)

	return savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex, grade
}

// NewRecord 创建-宠物
func NewRecord(pet *gameconfig.PetEntry, petUUID uint64, level uint32, grade pb.PetGrade) *pb.PetRecord {
	expMin, _ := gameconfig.GGameConfig.Exp.GetLevelMinExp(level)
	savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex, actualGrade := create(pet, level, grade)
	record := &pb.PetRecord{
		Uuid:               petUUID,
		AssetId:            *pet.ID,
		CarryStatus:        pb.PetCarryStatus_PetCarryStatus_Rest,
		Grade:              actualGrade,
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
	}
	recordGrowthBaseline(record, level)
	return record
}
