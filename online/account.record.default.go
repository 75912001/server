package main

import (
	"errors"
	"fmt"

	pb "server/proto/pb"
)

const (
	defaultCharacterID             uint32 = 1000011
	defaultCharacterName                  = "吉米"
	maxCharacterSlotCount          uint32 = uint32(pb.AccountRecordLimit_AccountRecordLimit_MaxCharacterSlotCount)
	maxPetCarryCount               int    = int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount)
	maxPetWarehouseCount           int    = int(pb.AccountRecordLimit_AccountRecordLimit_MaxPetWarehouseCount)
	defaultHP                      uint64 = 10
	defaultCharacterAttribute      uint64 = 10
	defaultCharacterAvailablePoint uint64 = 0
	defaultPetLoyalty              uint64 = 100
)

var (
	errCharacterSlotIndexInvalid = errors.New("character slot index invalid")
	errCharacterSlotOccupied     = errors.New("character slot occupied")
)

var defaultPetRecords = []struct {
	assetID uint32
	nick    string
	exp     uint64
}{
	{assetID: 4000101, nick: "利则诺顿", exp: 0},
	{assetID: 4000102, nick: "扬奇洛斯", exp: 0},
}

func initializeDefaultAccountRecord(record *pb.AccountRecord, characterSlotIndex uint32, now int64) error {
	if record == nil {
		return fmt.Errorf("account record is nil")
	}
	if record.GetUid() == 0 || record.GetAccount() == "" || record.GetAccountCreateTimestampMs() == 0 {
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

	if record.GetAccountRecordCreateTimestampMs() == 0 {
		record.AccountRecordCreateTimestampMs = now
	}
	if record.PetWarehouseRecordMap == nil {
		record.PetWarehouseRecordMap = make(map[uint64]*pb.PetRecord)
	}
	record.CharacterRecordList[slotIndex] = newDefaultCharacterRecord(record, now)
	return nil
}

func newDefaultCharacterRecord(record *pb.AccountRecord, now int64) *pb.CharacterRecord {
	characterUUID := nextAccountRecordUUID(record)
	character := &pb.CharacterRecord{
		Uuid:    characterUUID,
		Nick:    defaultCharacterName,
		AssetId: uint64(defaultCharacterID),
		AssetIdRecordMap: map[uint32]uint64{
			uint32(pb.AssetIDRecord_AssetIDRecord_HP):                             defaultHP,
			uint32(pb.AssetIDRecord_AssetIDRecord_CreateTimestamp):                uint64(now),
			uint32(pb.AssetIDRecord_AssetIDRecord_ElementalEarth):                 10,
			uint32(pb.AssetIDRecord_AssetIDRecord_ElementalWater):                 0,
			uint32(pb.AssetIDRecord_AssetIDRecord_ElementalFire):                  0,
			uint32(pb.AssetIDRecord_AssetIDRecord_ElementalWind):                  0,
			uint32(pb.AssetIDRecord_AssetIDRecord_Character_Available_Point):      defaultCharacterAvailablePoint,
			uint32(pb.AssetIDRecord_AssetIDRecord_Character_Attributes_Vitality):  defaultCharacterAttribute,
			uint32(pb.AssetIDRecord_AssetIDRecord_Character_Attributes_Strength):  defaultCharacterAttribute,
			uint32(pb.AssetIDRecord_AssetIDRecord_Character_Attributes_Toughness): defaultCharacterAttribute,
			uint32(pb.AssetIDRecord_AssetIDRecord_Character_Attributes_Dexterity): defaultCharacterAttribute,
		},
		RecordMap:    make(map[uint64]*pb.RecordPrimary),
		PetRecordMap: make(map[uint64]*pb.PetRecord),
	}

	for index, pet := range defaultPetRecords {
		if index == 0 {
			petRecord := newDefaultPetRecord(record, pet.assetID, pet.nick, pet.exp, pb.PetCarryStatus_PetCarryStatus_Battle, now)
			character.PetRecordMap[petRecord.GetUuid()] = petRecord
			continue
		}
		petRecord := newDefaultPetRecord(record, pet.assetID, pet.nick, pet.exp, pb.PetCarryStatus_PetCarryStatus_Rest, now)
		record.PetWarehouseRecordMap[petRecord.GetUuid()] = petRecord
	}
	return character
}

func newDefaultPetRecord(record *pb.AccountRecord, assetID uint32, nick string, exp uint64, carryStatus pb.PetCarryStatus, now int64) *pb.PetRecord {
	petUUID := nextAccountRecordUUID(record)
	return &pb.PetRecord{
		Uuid:        petUUID,
		Nick:        nick,
		CarryStatus: carryStatus,
		AssetRecordBaseMap: map[uint32]uint64{
			uint32(pb.AssetIDRecord_AssetIDRecord_AssetID):         uint64(assetID),
			uint32(pb.AssetIDRecord_AssetIDRecord_Exp):             exp,
			uint32(pb.AssetIDRecord_AssetIDRecord_CreateTimestamp): uint64(now),
			uint32(pb.AssetIDRecord_AssetIDRecord_Pet_Loyalty):     defaultPetLoyalty,
		},
		RecordMap: make(map[uint64]*pb.RecordPrimary),
	}
}

func nextAccountRecordUUID(record *pb.AccountRecord) uint64 {
	record.UsedUuid++
	return record.GetUsedUuid()
}
