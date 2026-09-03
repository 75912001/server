package gameconfig

import (
	"fmt"
	"math"
	"server/proto/pb"
)

const (
	petPanelAttributeHP = iota
	petPanelAttributeAttack
	petPanelAttributeDefense
	petPanelAttributeAgility
	petPanelAttributeCount
)

type PetPanelAttributeEntry struct {
	HP      *uint32 `yaml:"hp"`
	Attack  *uint32 `yaml:"attack"`
	Defense *uint32 `yaml:"defense"`
	Agility *uint32 `yaml:"agility"`
}

type PetPanelReferenceEntry struct {
	Level1CommonAverage   *PetPanelAttributeEntry `yaml:"level1CommonAverage"`
	Level1MythicAverage   *PetPanelAttributeEntry `yaml:"level1MythicAverage"`
	Level140CommonAverage *PetPanelAttributeEntry `yaml:"level140CommonAverage"`
	Level140MythicAverage *PetPanelAttributeEntry `yaml:"level140MythicAverage"`
	GrowthRateMin         *float64                `yaml:"growthRateMin"`
	GrowthRateMax         *float64                `yaml:"growthRateMax"`
}

// CalculatePetPanelHP 将服务端权威有符号Raw四维换算为面板耐久.
func CalculatePetPanelHP(rawVital int32, rawStr int32, rawTough int32, rawDex int32) int32 {
	return int32((float64(rawVital)*4.0 + float64(rawStr) + float64(rawTough) + float64(rawDex)) * 0.01)
}

// CalculatePetPanelAttack 将服务端权威有符号Raw四维换算为面板攻击.
func CalculatePetPanelAttack(rawVital int32, rawStr int32, rawTough int32, rawDex int32) int32 {
	return int32(float64(rawStr)*0.01 + float64(rawTough)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

// CalculatePetPanelDefense 将服务端权威有符号Raw四维换算为面板防御.
func CalculatePetPanelDefense(rawVital int32, rawStr int32, rawTough int32, rawDex int32) int32 {
	return int32(float64(rawTough)*0.01 + float64(rawStr)*0.001 + float64(rawVital)*0.001 + float64(rawDex)*0.0005)
}

// CalculatePetPanelAgility 将服务端权威有符号Raw速度换算为面板敏捷.
func CalculatePetPanelAgility(rawDex int32) int32 {
	return int32(float64(rawDex) * 0.01)
}

// PetRankGrowthRange 返回正式宠物升级使用的 Rank 随机成长倍率闭区间.
func PetRankGrowthRange(rank uint32) (float64, float64) {
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
		panic(fmt.Sprintf("pet rank is out of range: %d", rank))
	}
}

func calculatePetPanelReference(pet *PetEntry) PetPanelReferenceEntry {
	rankMin, rankMax := PetRankGrowthRange(pet.Growth.Rank)
	rankAverage := (rankMin + rankMax) / 2.0
	reference := PetPanelReferenceEntry{
		Level1CommonAverage:   calculatePetPanelAverage(pet.Growth, 1, petSavedBaseGradeOffsetMin, rankAverage),
		Level1MythicAverage:   calculatePetPanelAverage(pet.Growth, 1, petSavedBaseGradeOffsetMax, rankAverage),
		Level140CommonAverage: calculatePetPanelAverage(pet.Growth, uint32(pb.LevelRange_LevelRange_Max), petSavedBaseGradeOffsetMin, rankAverage),
		Level140MythicAverage: calculatePetPanelAverage(pet.Growth, uint32(pb.LevelRange_LevelRange_Max), petSavedBaseGradeOffsetMax, rankAverage),
	}
	reference.GrowthRateMin = valuePtr(calculatePetPanelGrowthRate(reference.Level1CommonAverage, reference.Level140CommonAverage))
	reference.GrowthRateMax = valuePtr(calculatePetPanelGrowthRate(reference.Level1MythicAverage, reference.Level140MythicAverage))
	return reference
}

// calculatePetPanelGrowthRate 沿用客户端总成长口径, 只合计攻击、防御和敏捷的每级平均成长.
func calculatePetPanelGrowthRate(level1 *PetPanelAttributeEntry, level140 *PetPanelAttributeEntry) float64 {
	growthTotal := (*level140.Attack - *level1.Attack) +
		(*level140.Defense - *level1.Defense) +
		(*level140.Agility - *level1.Agility)
	rate := float64(growthTotal) / float64(pb.LevelRange_LevelRange_Max-1)
	return math.Round(rate*1000.0) / 1000.0
}

func calculatePetPanelAverage(growth *PetGrowthEntry, level uint32, gradeOffset int32, rankAverage float64) *PetPanelAttributeEntry {
	savedBase := petSavedBase(growth, gradeOffset)
	initialRaw := averagePetPanelRaw(savedBase, float64(*growth.InitNum))
	upgradeRaw := averagePetPanelRaw(savedBase, rankAverage)
	for rawIndex := 0; rawIndex < petPanelAttributeCount; rawIndex++ {
		initialRaw[rawIndex] += int32(int64(upgradeRaw[rawIndex]) * int64(level-1))
	}
	return newPetPanelAttributeEntry([petPanelAttributeCount]uint32{
		uint32(CalculatePetPanelHP(initialRaw[0], initialRaw[1], initialRaw[2], initialRaw[3])),
		uint32(CalculatePetPanelAttack(initialRaw[0], initialRaw[1], initialRaw[2], initialRaw[3])),
		uint32(CalculatePetPanelDefense(initialRaw[0], initialRaw[1], initialRaw[2], initialRaw[3])),
		uint32(CalculatePetPanelAgility(initialRaw[3])),
	})
}

func averagePetPanelRaw(savedBase [petPanelAttributeCount]int32, multiplier float64) [petPanelAttributeCount]int32 {
	distributions := [][petPanelAttributeCount]uint32{
		{2, 3, 3, 2},
		{2, 2, 3, 3},
		{3, 2, 2, 3},
		{3, 3, 2, 2},
	}
	totals := [petPanelAttributeCount]int64{}
	for _, distribution := range distributions {
		raw := petPanelRaw(savedBase, distribution, multiplier)
		for rawIndex := 0; rawIndex < petPanelAttributeCount; rawIndex++ {
			totals[rawIndex] += int64(raw[rawIndex])
		}
	}
	average := [petPanelAttributeCount]int32{}
	for rawIndex := 0; rawIndex < petPanelAttributeCount; rawIndex++ {
		average[rawIndex] = int32(totals[rawIndex] / int64(len(distributions)))
	}
	return average
}

func petSavedBase(growth *PetGrowthEntry, gradeOffset int32) [petPanelAttributeCount]int32 {
	return [petPanelAttributeCount]int32{
		int32(*growth.BaseVital) + gradeOffset,
		int32(*growth.BaseStr) + gradeOffset,
		int32(*growth.BaseTough) + gradeOffset,
		int32(*growth.BaseDex) + gradeOffset,
	}
}

func petPanelRaw(savedBase [petPanelAttributeCount]int32, distribution [petPanelAttributeCount]uint32, multiplier float64) [petPanelAttributeCount]int32 {
	raw := [petPanelAttributeCount]int32{}
	for index := 0; index < petPanelAttributeCount; index++ {
		raw[index] = int32(float64(savedBase[index]+int32(distribution[index])) * multiplier)
	}
	return raw
}

func newPetPanelAttributeEntry(values [petPanelAttributeCount]uint32) *PetPanelAttributeEntry {
	return &PetPanelAttributeEntry{
		HP:      valuePtr(values[petPanelAttributeHP]),
		Attack:  valuePtr(values[petPanelAttributeAttack]),
		Defense: valuePtr(values[petPanelAttributeDefense]),
		Agility: valuePtr(values[petPanelAttributeAgility]),
	}
}

func petPanelReferenceEqual(actual *PetPanelReferenceEntry, expected *PetPanelReferenceEntry) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	return petPanelAttributeEqual(actual.Level1CommonAverage, expected.Level1CommonAverage) &&
		petPanelAttributeEqual(actual.Level1MythicAverage, expected.Level1MythicAverage) &&
		petPanelAttributeEqual(actual.Level140CommonAverage, expected.Level140CommonAverage) &&
		petPanelAttributeEqual(actual.Level140MythicAverage, expected.Level140MythicAverage) &&
		float64PointerEqual(actual.GrowthRateMin, expected.GrowthRateMin) &&
		float64PointerEqual(actual.GrowthRateMax, expected.GrowthRateMax)
}

func petPanelAttributeEqual(actual *PetPanelAttributeEntry, expected *PetPanelAttributeEntry) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	return uint32PointerEqual(actual.HP, expected.HP) &&
		uint32PointerEqual(actual.Attack, expected.Attack) &&
		uint32PointerEqual(actual.Defense, expected.Defense) &&
		uint32PointerEqual(actual.Agility, expected.Agility)
}

func uint32PointerEqual(actual *uint32, expected *uint32) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	return *actual == *expected
}

func float64PointerEqual(actual *float64, expected *float64) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	return *actual == *expected
}

func formatPetPanelReferenceError(pet *PetEntry, expected *PetPanelReferenceEntry) string {
	return fmt.Sprintf("pet:%d name:%s panelReference不一致\nactual:\n%s\nexpected:\n%s",
		*pet.ID, *pet.Name, formatPetPanelReference(pet.PanelReference), formatPetPanelReference(expected))
}

func formatPetPanelReference(reference *PetPanelReferenceEntry) string {
	if reference == nil {
		return "      panelReference: <missing>"
	}
	return fmt.Sprintf("      panelReference:\n%s\n%s\n%s\n%s\n        growthRateMin: %s\n        growthRateMax: %s",
		formatPetPanelAttribute("level1CommonAverage", reference.Level1CommonAverage),
		formatPetPanelAttribute("level1MythicAverage", reference.Level1MythicAverage),
		formatPetPanelAttribute("level140CommonAverage", reference.Level140CommonAverage),
		formatPetPanelAttribute("level140MythicAverage", reference.Level140MythicAverage),
		formatFloat64Pointer(reference.GrowthRateMin), formatFloat64Pointer(reference.GrowthRateMax))
}

func formatPetPanelAttribute(key string, attribute *PetPanelAttributeEntry) string {
	if attribute == nil {
		return fmt.Sprintf("        %s: <missing>", key)
	}
	return fmt.Sprintf("        %s: {hp: %s, attack: %s, defense: %s, agility: %s}", key,
		formatUint32Pointer(attribute.HP), formatUint32Pointer(attribute.Attack),
		formatUint32Pointer(attribute.Defense), formatUint32Pointer(attribute.Agility))
}

func formatUint32Pointer(value *uint32) string {
	if value == nil {
		return "<missing>"
	}
	return fmt.Sprintf("%d", *value)
}

func formatFloat64Pointer(value *float64) string {
	if value == nil {
		return "<missing>"
	}
	return fmt.Sprintf("%.3f", *value)
}
