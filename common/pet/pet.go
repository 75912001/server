package pet

import (
	"errors"
	"fmt"
	"server/common/gameconfig"
	"server/proto/pb"
	"time"

	xutil "github.com/75912001/xlib/util"
)

// ErrOrdinaryCreationUnsupported 表示宠物模板必须使用普通创建以外的专用生成流程.
var ErrOrdinaryCreationUnsupported = errors.New("ordinary pet creation is unsupported")

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
	newRawVitality, newRawStrength, newRawToughness, newRawDexterity, err := upgrade(
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
	if err != nil {
		return err
	}
	if _, _, _, _, err = calculatePetPanelAttributes(newRawVitality, newRawStrength, newRawToughness, newRawDexterity); err != nil {
		return fmt.Errorf("upgraded pet attribute is invalid: pet:%d: %w", record.GetAssetId(), err)
	}
	record.RawVitality = newRawVitality
	record.RawStrength = newRawStrength
	record.RawToughness = newRawToughness
	record.RawDexterity = newRawDexterity
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
	if _, _, _, _, err := calculatePetPanelAttributes(
		record.GetRawVitality(),
		record.GetRawStrength(),
		record.GetRawToughness(),
		record.GetRawDexterity(),
	); err != nil {
		return err
	}
	if currentLevel == uint32(pb.LevelRange_LevelRange_Min) {
		return recordGrowthBaseline(record, currentLevel)
	}
	if baselineLevel := record.GetGrowthBaselineLevel(); baselineLevel != 0 {
		if baselineLevel > currentLevel || baselineLevel > uint32(pb.LevelRange_LevelRange_Max) {
			return fmt.Errorf("pet growth baseline level %d exceeds current level %d", baselineLevel, currentLevel)
		}
		return nil
	}
	return recordGrowthBaseline(record, currentLevel)
}

func recordGrowthBaseline(record *pb.PetRecord, level uint32) error {
	hp, attack, defense, agility, err := calculatePetPanelAttributes(
		record.GetRawVitality(),
		record.GetRawStrength(),
		record.GetRawToughness(),
		record.GetRawDexterity(),
	)
	if err != nil {
		return err
	}
	record.GrowthBaselineHp = hp
	record.GrowthBaselineAttack = attack
	record.GrowthBaselineDefense = defense
	record.GrowthBaselineAgility = agility
	record.GrowthBaselineLevel = level
	return nil
}

// calculatePetPanelAttributes在有符号Raw四维完成本地计算后, 只校验协议仍要求为正数的HP和攻击.
// 防御和敏捷本身就是int32协议字段, 允许原版低基础四维生成0或负数.
func calculatePetPanelAttributes(rawVital int32, rawStr int32, rawTough int32, rawDex int32) (
	hp uint32,
	attack uint32,
	defense int32,
	agility int32,
	err error,
) {
	calculatedHP := gameconfig.CalculatePetPanelHP(rawVital, rawStr, rawTough, rawDex)
	if calculatedHP <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("pet panel HP must be positive: value:%d", calculatedHP)
	}
	calculatedAttack := gameconfig.CalculatePetPanelAttack(rawVital, rawStr, rawTough, rawDex)
	if calculatedAttack <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("pet panel attack must be positive: value:%d", calculatedAttack)
	}
	return uint32(calculatedHP),
		uint32(calculatedAttack),
		gameconfig.CalculatePetPanelDefense(rawVital, rawStr, rawTough, rawDex),
		gameconfig.CalculatePetPanelAgility(rawDex),
		nil
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
	savedBaseVital int32,
	savedBaseStr int32,
	savedBaseTough int32,
	savedBaseDex int32,
	rawVital int32,
	rawStr int32,
	rawTough int32,
	rawDex int32) (newRawVital int32,
	newRawStr int32,
	newRawTough int32,
	newRawDex int32,
	err error) {
	if upgradeCount == 0 {
		// 零次升级必须保留调用方传入的当前 Raw, 避免 1 级宠物的初始属性被清零.
		return rawVital, rawStr, rawTough, rawDex, nil
	}
	rankMin, rankMax := gameconfig.PetRankGrowthRange(pet.Growth.Rank)
	savedBases := [4]int32{savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex}
	rawValues := [4]int32{rawVital, rawStr, rawTough, rawDex}
	for i := uint32(0); i < upgradeCount; i++ {
		randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
		randomPoints := [4]uint32{randomVital, randomStr, randomTough, randomDex}
		const randomPrecision = uint64(1_000_000_000)
		randomRatio := float64(xutil.RandomU64(0, randomPrecision)) / float64(randomPrecision)
		rankRand := rankMin + randomRatio*(rankMax-rankMin)
		for index := range rawValues {
			delta, scaleErr := scalePetRawAttribute(savedBases[index], randomPoints[index], rankRand)
			if scaleErr != nil {
				return 0, 0, 0, 0, fmt.Errorf("upgrade pet raw attribute failed index:%d: %w", index, scaleErr)
			}
			nextValue := int64(rawValues[index]) + int64(delta)
			if nextValue < int64(-1<<31) || nextValue > int64(1<<31-1) {
				return 0, 0, 0, 0, fmt.Errorf("upgraded pet raw attribute overflows int32 index:%d value:%d", index, nextValue)
			}
			rawValues[index] = int32(nextValue)
		}
	}
	return rawValues[0], rawValues[1], rawValues[2], rawValues[3], nil
}

func scalePetRawAttribute(savedBase int32, randomPoint uint32, multiplier float64) (int32, error) {
	base := int64(savedBase) + int64(randomPoint)
	value := float64(base) * multiplier
	if value < float64(-1<<31) || value > float64(1<<31-1) {
		return 0, fmt.Errorf("pet raw attribute overflows int32: base:%d multiplier:%v value:%v", base, multiplier, value)
	}
	return int32(value), nil
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
	savedBaseVital int32,
	savedBaseStr int32,
	savedBaseTough int32,
	savedBaseDex int32,
	rawVital int32,
	rawStr int32,
	rawTough int32,
	rawDex int32,
	actualGrade pb.PetGrade,
	err error) {
	var vitalOffset, strOffset, toughOffset, dexOffset int32
	if grade == pb.PetGrade_PetGrade_Unknow {
		// 未指定品阶时四维独立随机, 再按四维总偏移计算并保存实际品阶.
		const randomOffsetRange = uint32(petSavedBaseOffsetMax - petSavedBaseOffsetMin)
		vitalOffset = int32(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		strOffset = int32(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		toughOffset = int32(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		dexOffset = int32(xutil.RandomU32(0, randomOffsetRange)) + petSavedBaseOffsetMin
		grade = petGradeFromRandomOffsetTotal(int(vitalOffset + strOffset + toughOffset + dexOffset))
	} else {
		var gradeOffset int32
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
		default:
			return 0, 0, 0, 0, 0, 0, 0, 0, pb.PetGrade_PetGrade_Unknow, fmt.Errorf("pet grade is invalid: %s", grade)
		}
		vitalOffset = gradeOffset
		strOffset = gradeOffset
		toughOffset = gradeOffset
		dexOffset = gradeOffset
	}

	templateBases := [4]uint32{*pet.Growth.BaseVital, *pet.Growth.BaseStr, *pet.Growth.BaseTough, *pet.Growth.BaseDex}
	offsets := [4]int32{vitalOffset, strOffset, toughOffset, dexOffset}
	savedBases := [4]int32{}
	for index := range savedBases {
		value := int64(templateBases[index]) + int64(offsets[index])
		if value < int64(-1<<31) || value > int64(1<<31-1) {
			return 0, 0, 0, 0, 0, 0, 0, 0, pb.PetGrade_PetGrade_Unknow,
				fmt.Errorf("pet saved base overflows int32 index:%d value:%d", index, value)
		}
		savedBases[index] = int32(value)
	}
	savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex = savedBases[0], savedBases[1], savedBases[2], savedBases[3]

	randomVital, randomStr, randomTough, randomDex := randomFourPointDistribution()
	initialFactor := float64(*pet.Growth.InitNum)
	randomPoints := [4]uint32{randomVital, randomStr, randomTough, randomDex}
	rawValues := [4]int32{}
	for index := range rawValues {
		rawValues[index], err = scalePetRawAttribute(savedBases[index], randomPoints[index], initialFactor)
		if err != nil {
			return 0, 0, 0, 0, 0, 0, 0, 0, pb.PetGrade_PetGrade_Unknow,
				fmt.Errorf("create pet raw attribute failed index:%d: %w", index, err)
		}
	}
	rawVital, rawStr, rawTough, rawDex, err = upgrade(
		pet,
		level-1,
		savedBaseVital,
		savedBaseStr,
		savedBaseTough,
		savedBaseDex,
		rawValues[0],
		rawValues[1],
		rawValues[2],
		rawValues[3],
	)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, pb.PetGrade_PetGrade_Unknow, err
	}
	return savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex, grade, nil
}

// NewRecord 创建-宠物
// 普通宠物按原版有符号四维创建; 融合蛋仍由独立流程根据亲本继承四维, 不使用模板占位值.
func NewRecord(pet *gameconfig.PetEntry, petUUID uint64, level uint32, grade pb.PetGrade) (*pb.PetRecord, error) {
	if pet == nil || pet.ID == nil {
		return nil, fmt.Errorf("pet config or id is empty")
	}
	if !pet.SupportsOrdinaryCreation() {
		return nil, fmt.Errorf("%w: pet:%d mode:%q", ErrOrdinaryCreationUnsupported, *pet.ID, pet.CreationMode)
	}
	if pet.Growth == nil || pet.Growth.InitNum == nil || pet.Growth.BaseVital == nil || pet.Growth.BaseStr == nil || pet.Growth.BaseTough == nil || pet.Growth.BaseDex == nil {
		return nil, fmt.Errorf("pet growth is incomplete: pet:%d", *pet.ID)
	}
	if level < uint32(pb.LevelRange_LevelRange_Min) || level > uint32(pb.LevelRange_LevelRange_Max) {
		return nil, fmt.Errorf("pet level is out of range: pet:%d level:%d", *pet.ID, level)
	}
	expMin, err := gameconfig.GGameConfig.Exp.GetLevelMinExp(level)
	if err != nil {
		return nil, err
	}
	savedBaseVital, savedBaseStr, savedBaseTough, savedBaseDex, rawVital, rawStr, rawTough, rawDex, actualGrade, err := create(pet, level, grade)
	if err != nil {
		return nil, err
	}
	if _, _, _, _, err = calculatePetPanelAttributes(rawVital, rawStr, rawTough, rawDex); err != nil {
		return nil, fmt.Errorf("created pet attribute is invalid: pet:%d: %w", *pet.ID, err)
	}
	record := &pb.PetRecord{
		Uuid:               petUUID,
		AssetId:            *pet.ID,
		CarryStatus:        pb.PetCarryStatus_PetCarryStatus_Rest,
		SkillIdList:        append([]uint32(nil), pet.SkillSlots...),
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
	if err := recordGrowthBaseline(record, level); err != nil {
		return nil, err
	}
	return record, nil
}
