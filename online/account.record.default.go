package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	pb "server/proto/pb"
)

const (
	defaultCharacterID             uint32 = 1000011
	defaultCharacterName                  = "吉米"
	maxCharacterNickRuneCount             = 12
	maxCharacterSlotCount          uint32 = uint32(pb.AccountRecordLimit_AccountRecordLimit_MaxCharacterSlotCount)
	maxPetCarryCount               int    = int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount)
	elementalTotalPoint            uint32 = uint32(pb.ElementalLimit_ElementalLimit_TotalPoint)
	elementalMaxTypeCount          int    = int(pb.ElementalLimit_ElementalLimit_MaxActiveTypeCount)
	characterAttributeTotalPoint   uint32 = uint32(pb.CharacterAttributeLimit_CharacterAttributeLimit_TotalPoint)
	defaultCharacterAvailablePoint uint32 = 0
	defaultCharacterSceneID        uint32 = 2000001
	defaultPetLoyalty              uint64 = 100
	defaultPetGrade                       = pb.PetGrade_PetGrade_Mythic
	unspecifiedPetGrade                   = pb.PetGrade_PetGrade_Epic
)

var (
	errCharacterSlotIndexInvalid = errors.New("character slot index invalid")
	errCharacterSlotOccupied     = errors.New("character slot occupied")
	errCharacterIDInvalid        = errors.New("character id invalid")
	errCharacterNickInvalid      = errors.New("character nick invalid")
	errCharacterElementalInvalid = errors.New("character elemental invalid")
	errCharacterAttributeInvalid = errors.New("character attribute invalid")
)

type elementalPoints struct {
	earth uint32
	water uint32
	fire  uint32
	wind  uint32
}

type characterAttributePoints struct {
	vitality  uint32
	strength  uint32
	toughness uint32
	dexterity uint32
}

var defaultPetRecords = []struct {
	assetID uint32
	nick    string
	exp     uint64
}{
	{assetID: 4000101, nick: "利则诺顿", exp: 0},
	{assetID: 4000102, nick: "扬奇洛斯", exp: 0},
	{assetID: 4000103, nick: "邦浦洛斯", exp: 0},
	{assetID: 4000104, nick: "邦奇诺", exp: 0},
	{assetID: 4000105, nick: "布鲁顿", exp: 0},
}

func initializeDefaultAccountRecord(record *pb.AccountRecord, characterSlotIndex uint32, characterID uint32, characterNick string, characterElemental *pb.ElementalPoints, characterAttribute *pb.CharacterAttributePoints, now int64) error {
	if record == nil {
		return fmt.Errorf("account record is nil")
	}
	if record.GetAid() == 0 || record.GetAccount() == "" || record.GetAccountCreateTimestampMs() == 0 {
		return fmt.Errorf("account record identity is invalid")
	}
	if characterSlotIndex >= maxCharacterSlotCount {
		return errCharacterSlotIndexInvalid
	}
	slotIndex := int(characterSlotIndex)
	for len(record.CharacterRecordList) <= slotIndex {
		record.CharacterRecordList = append(record.CharacterRecordList, &pb.CharacterRecord{})
	}

	if character := record.CharacterRecordList[slotIndex]; character != nil && character.GetUuid() != 0 {
		return errCharacterSlotOccupied
	}

	resolvedCharacterID := normalizeCharacterID(characterID)
	if !isCreatableCharacterID(resolvedCharacterID) {
		return errCharacterIDInvalid
	}
	resolvedCharacterNick := normalizeCharacterNick(characterNick)
	if !isValidCharacterNick(resolvedCharacterNick) {
		return errCharacterNickInvalid
	}
	resolvedCharacterElemental, err := normalizeElemental(characterElemental)
	if err != nil {
		return err
	}
	resolvedCharacterAttribute, err := normalizeCharacterAttribute(characterAttribute)
	if err != nil {
		return err
	}

	if record.GetAccountRecordCreateTimestampMs() == 0 {
		record.AccountRecordCreateTimestampMs = now
	}
	if record.PetWarehouseRecordMap == nil {
		record.PetWarehouseRecordMap = make(map[uint64]*pb.PetRecord)
	}
	character, err := newDefaultCharacterRecord(record, resolvedCharacterID, resolvedCharacterNick, resolvedCharacterElemental, resolvedCharacterAttribute, now)
	if err != nil {
		return err
	}
	record.CharacterRecordList[slotIndex] = character
	return nil
}

func newDefaultCharacterRecord(record *pb.AccountRecord, characterID uint32, characterNick string, elemental elementalPoints, attribute characterAttributePoints, now int64) (*pb.CharacterRecord, error) {
	characterUUID := nextAccountRecordUUID(record)
	character := &pb.CharacterRecord{
		Uuid:              characterUUID,
		Nick:              characterNick,
		AssetId:           uint64(characterID),
		Exp:               0,
		Elemental:         newCharacterElementalPoints(elemental),
		AvailablePoint:    defaultCharacterAvailablePoint,
		Attribute:         newCharacterAttributePoints(attribute),
		CreateTimestampMs: now,
		SceneId:           defaultCharacterSceneID,
		RecordMap:         make(map[uint64]*pb.RecordPrimary),
		PetRecordList:     make([]*pb.PetRecord, 0, maxPetCarryCount),
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(record.GetAid())))
	if len(defaultPetRecords) > maxPetCarryCount {
		return nil, fmt.Errorf("default pet count %d exceeds max carry count %d", len(defaultPetRecords), maxPetCarryCount)
	}
	for index, pet := range defaultPetRecords {
		carryStatus := pb.PetCarryStatus_PetCarryStatus_Wait
		if index == 0 {
			carryStatus = pb.PetCarryStatus_PetCarryStatus_Battle
		}
		petRecord, err := newDefaultPetRecord(record, pet.assetID, pet.nick, pet.exp, defaultPetGrade, carryStatus, now, rng)
		if err != nil {
			return nil, err
		}
		character.PetRecordList = append(character.PetRecordList, petRecord)
	}
	return character, nil
}

func newCharacterElementalPoints(elemental elementalPoints) *pb.ElementalPoints {
	return &pb.ElementalPoints{
		Earth: elemental.earth,
		Water: elemental.water,
		Fire:  elemental.fire,
		Wind:  elemental.wind,
	}
}

func newCharacterAttributePoints(attribute characterAttributePoints) *pb.CharacterAttributePoints {
	return &pb.CharacterAttributePoints{
		Vitality:  attribute.vitality,
		Strength:  attribute.strength,
		Toughness: attribute.toughness,
		Dexterity: attribute.dexterity,
	}
}

func normalizeCharacterID(characterID uint32) uint32 {
	if characterID == 0 {
		return defaultCharacterID
	}
	return characterID
}

func normalizeCharacterNick(characterNick string) string {
	trimmedNick := strings.TrimSpace(characterNick)
	if trimmedNick == "" {
		return defaultCharacterName
	}
	return trimmedNick
}

func isValidCharacterNick(characterNick string) bool {
	nickRuneCount := utf8.RuneCountInString(characterNick)
	return nickRuneCount > 0 && nickRuneCount <= maxCharacterNickRuneCount
}

func normalizeElemental(elemental *pb.ElementalPoints) (elementalPoints, error) {
	if elemental == nil {
		return elementalPoints{}, errCharacterElementalInvalid
	}

	points := elementalPoints{
		earth: elemental.GetEarth(),
		water: elemental.GetWater(),
		fire:  elemental.GetFire(),
		wind:  elemental.GetWind(),
	}
	if !isValidElementalAllocation(points) {
		return elementalPoints{}, errCharacterElementalInvalid
	}
	return points, nil
}

func isValidElementalAllocation(points elementalPoints) bool {
	values := []uint32{points.earth, points.water, points.fire, points.wind}
	activeIndexes := make([]int, 0, elementalMaxTypeCount)
	total := uint32(0)
	for index, value := range values {
		if value > elementalTotalPoint {
			return false
		}
		total += value
		if value > 0 {
			activeIndexes = append(activeIndexes, index)
			if len(activeIndexes) > elementalMaxTypeCount {
				return false
			}
		}
	}
	if total != elementalTotalPoint {
		return false
	}

	activeCount := len(activeIndexes)
	if activeCount == 1 {
		return true
	}
	if activeCount != 2 {
		return false
	}

	diff := activeIndexes[0] - activeIndexes[1]
	if diff < 0 {
		diff = -diff
	}
	return diff == 1 || diff == len(values)-1
}

func normalizeCharacterAttribute(attribute *pb.CharacterAttributePoints) (characterAttributePoints, error) {
	if attribute == nil {
		return characterAttributePoints{}, errCharacterAttributeInvalid
	}

	points := characterAttributePoints{
		vitality:  attribute.GetVitality(),
		strength:  attribute.GetStrength(),
		toughness: attribute.GetToughness(),
		dexterity: attribute.GetDexterity(),
	}
	if !isValidCharacterAttributeAllocation(points) {
		return characterAttributePoints{}, errCharacterAttributeInvalid
	}
	return points, nil
}

func isValidCharacterAttributeAllocation(points characterAttributePoints) bool {
	values := []uint32{points.vitality, points.strength, points.toughness, points.dexterity}
	total := uint32(0)
	for _, value := range values {
		if value > characterAttributeTotalPoint {
			return false
		}
		total += value
	}
	return total == characterAttributeTotalPoint
}

func isCreatableCharacterID(characterID uint32) bool {
	if GGameConfig == nil || GGameConfig.Character == nil {
		return false
	}
	entry := GGameConfig.Character.GetByID(int(characterID))
	return entry != nil && entry.IsRole
}

func newDefaultPetRecord(record *pb.AccountRecord, assetID uint32, nick string, exp uint64, grade pb.PetGrade, carryStatus pb.PetCarryStatus, now int64, rng *rand.Rand) (*pb.PetRecord, error) {
	level := 1
	if GGameConfig != nil && GGameConfig.Exp != nil {
		resolvedLevel, err := GGameConfig.Exp.GetLevel(int(exp))
		if err != nil {
			return nil, err
		}
		level = resolvedLevel
	}
	attribute, err := createPetRuntimeAttribute(assetID, level, grade, rng)
	if err != nil {
		return nil, err
	}
	petUUID := nextAccountRecordUUID(record)
	assetRecordBaseMap := map[uint32]uint64{
		uint32(pb.AssetIDRecord_AssetIDRecord_AssetID): uint64(assetID),
	}
	return &pb.PetRecord{
		Uuid:               petUUID,
		Nick:               nick,
		CarryStatus:        carryStatus,
		Grade:              grade,
		Exp:                exp,
		Loyalty:            defaultPetLoyalty,
		SavedBaseVitality:  attribute.savedBaseVital,
		SavedBaseStrength:  attribute.savedBaseStr,
		SavedBaseToughness: attribute.savedBaseTough,
		SavedBaseDexterity: attribute.savedBaseDex,
		RawVitality:        attribute.rawVital,
		RawStrength:        attribute.rawStr,
		RawToughness:       attribute.rawTough,
		RawDexterity:       attribute.rawDex,
		CreateTimestampMs:  now,
		AssetRecordBaseMap: assetRecordBaseMap,
		RecordMap:          make(map[uint64]*pb.RecordPrimary),
	}, nil
}

func nextAccountRecordUUID(record *pb.AccountRecord) uint64 {
	record.UsedUuid++
	return record.GetUsedUuid()
}
