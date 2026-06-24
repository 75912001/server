package main

import (
	"fmt"

	pb "server/proto/pb"
)

const (
	defaultCharacterID             uint32 = 1000011
	defaultCharacterName                  = "吉米"
	defaultHP                      uint64 = 10
	defaultCharacterAttribute      uint64 = 10
	defaultCharacterAvailablePoint uint64 = 0
	defaultPetLoyalty              uint64 = 100
)

var defaultPetRecords = []struct {
	assetID uint32
	nick    string
	exp     uint64
}{
	{assetID: 4000101, nick: "利则诺顿", exp: 0},
	{assetID: 4000102, nick: "扬奇洛斯", exp: 0},
}

func initializeDefaultAccountRecord(record *pb.AccountRecord, now int64) error {
	if record == nil {
		return fmt.Errorf("account record is nil")
	}
	if record.GetUid() == 0 || record.GetAccount() == "" || record.GetAccountCreateTimestampMs() == 0 {
		return fmt.Errorf("account record identity is invalid")
	}

	record.AccountRecordCreateTimestampMs = now
	if record.CharacterRecordMap == nil {
		record.CharacterRecordMap = make(map[uint64]*pb.CharacterRecord)
	}
	if len(record.CharacterRecordMap) == 0 {
		character := newDefaultCharacterRecord(record, now)
		record.CharacterRecordMap[character.GetUuid()] = character
	}
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

	for _, pet := range defaultPetRecords {
		petRecord := newDefaultPetRecord(record, pet.assetID, pet.nick, pet.exp, now)
		character.PetRecordMap[petRecord.GetUuid()] = petRecord
	}
	return character
}

func newDefaultPetRecord(record *pb.AccountRecord, assetID uint32, nick string, exp uint64, now int64) *pb.PetRecord {
	petUUID := nextAccountRecordUUID(record)
	return &pb.PetRecord{
		Uuid: petUUID,
		Nick: nick,
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
