package main

import (
	"fmt"
	"strings"

	pb "server/proto/pb"

	"google.golang.org/protobuf/proto"
)

// validateAccountRecord 校验 cache 返回的账号档案是否完整符合当前协议和业务结构.
// 项目不兼容旧档案, 因此缺少当前直字段或仍使用旧数据布局时直接拒绝绑定, 不迁移也不回填.
func validateAccountRecord(record *pb.AccountRecord) error {
	if record == nil {
		return fmt.Errorf("record is nil")
	}
	if record.GetCreateTimestampMs() <= 0 {
		return fmt.Errorf("create timestamp is invalid")
	}
	if len(record.GetCharacterRecordList()) != int(pb.AccountRecordLimit_AccountRecordLimit_MaxCharacterSlotCount) {
		return fmt.Errorf("character slot count is invalid")
	}
	if len(record.GetPetWarehouseRecordMap()) > int(pb.AccountRecordLimit_AccountRecordLimit_MaxPetWarehouseCount) {
		return fmt.Errorf("pet warehouse count exceeds limit")
	}

	usedUUID := record.GetUsedUuid()
	seenUUID := make(map[uint64]struct{})
	for slot, characterRecord := range record.GetCharacterRecordList() {
		if characterRecord == nil {
			return fmt.Errorf("character slot %d is nil", slot)
		}
		characterUUID := characterRecord.GetUuid()
		if characterUUID == 0 {
			if proto.Size(characterRecord) != 0 {
				return fmt.Errorf("empty character slot %d contains data", slot)
			}
			continue
		}
		if err := registerAccountRecordUUID(seenUUID, usedUUID, characterUUID); err != nil {
			return fmt.Errorf("character slot %d: %w", slot, err)
		}
		if err := validateCharacterRecord(characterRecord, seenUUID, usedUUID); err != nil {
			return fmt.Errorf("character slot %d: %w", slot, err)
		}
	}

	for petUUID, petRecord := range record.GetPetWarehouseRecordMap() {
		if petRecord == nil {
			return fmt.Errorf("warehouse pet %d is nil", petUUID)
		}
		if petUUID == 0 || petRecord.GetUuid() != petUUID {
			return fmt.Errorf("warehouse pet key %d does not match uuid %d", petUUID, petRecord.GetUuid())
		}
		if err := registerAccountRecordUUID(seenUUID, usedUUID, petUUID); err != nil {
			return fmt.Errorf("warehouse pet %d: %w", petUUID, err)
		}
		if err := validatePetRecord(petRecord, true); err != nil {
			return fmt.Errorf("warehouse pet %d: %w", petUUID, err)
		}
	}
	return nil
}

func validateCharacterRecord(record *pb.CharacterRecord, seenUUID map[uint64]struct{}, usedUUID uint64) error {
	if strings.TrimSpace(record.GetNick()) == "" {
		return fmt.Errorf("nick is empty")
	}
	if !assetIDInRange(record.GetAssetId(), pb.AssetIDRange_AssetIDRange_Character_Start, pb.AssetIDRange_AssetIDRange_Character_End) {
		return fmt.Errorf("asset id %d is invalid", record.GetAssetId())
	}
	if record.GetCreateTimestampMs() <= 0 {
		return fmt.Errorf("create timestamp is invalid")
	}
	if !assetIDInRange(uint64(record.GetSceneId()), pb.AssetIDRange_AssetIDRange_Scene_Start, pb.AssetIDRange_AssetIDRange_Scene_End) {
		return fmt.Errorf("scene id %d is invalid", record.GetSceneId())
	}
	if record.GetVitality()+record.GetStrength()+record.GetToughness()+record.GetDexterity() == 0 {
		return fmt.Errorf("attribute is empty")
	}
	if err := validateCharacterLuckState(record.GetLuckState()); err != nil {
		return fmt.Errorf("luck state: %w", err)
	}
	if record.GetLuckState().GetBaseLuck() == 0 && (record.GetLastLoginTimestampMs() != 0 || record.GetLastLogoutTimestampMs() != 0) {
		return fmt.Errorf("pending luck state has login history")
	}
	if len(record.GetPetRecordList()) > int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return fmt.Errorf("carried pet count exceeds limit")
	}

	battleCount := 0
	mountCount := 0
	for index, petRecord := range record.GetPetRecordList() {
		if petRecord == nil {
			return fmt.Errorf("carried pet %d is nil", index)
		}
		petUUID := petRecord.GetUuid()
		if err := registerAccountRecordUUID(seenUUID, usedUUID, petUUID); err != nil {
			return fmt.Errorf("carried pet %d: %w", index, err)
		}
		if err := validatePetRecord(petRecord, false); err != nil {
			return fmt.Errorf("carried pet %d: %w", index, err)
		}
		switch petRecord.GetCarryStatus() {
		case pb.PetCarryStatus_PetCarryStatus_Battle:
			battleCount++
		case pb.PetCarryStatus_PetCarryStatus_Mount:
			mountCount++
		}
	}
	if battleCount > 1 {
		return fmt.Errorf("multiple battle pets")
	}
	if mountCount > 1 {
		return fmt.Errorf("multiple mount pets")
	}
	return nil
}

func validatePetRecord(record *pb.PetRecord, warehouse bool) error {
	if record.GetUuid() == 0 {
		return fmt.Errorf("uuid is empty")
	}
	if !assetIDInRange(record.GetAssetId(), pb.AssetIDRange_AssetIDRange_Pet_Start, pb.AssetIDRange_AssetIDRange_Pet_End) {
		return fmt.Errorf("asset id %d is invalid", record.GetAssetId())
	}
	if record.GetGrade() <= pb.PetGrade_PetGrade_Unknow || record.GetGrade() >= pb.PetGrade_PetGrade_Max {
		return fmt.Errorf("grade %s is invalid", record.GetGrade())
	}
	if record.GetCarryStatus() <= pb.PetCarryStatus_PetCarryStatus_Unknow || record.GetCarryStatus() >= pb.PetCarryStatus_PetCarryStatus_Max {
		return fmt.Errorf("carry status %s is invalid", record.GetCarryStatus())
	}
	if warehouse && record.GetCarryStatus() != pb.PetCarryStatus_PetCarryStatus_Rest {
		return fmt.Errorf("warehouse pet carry status must be Rest")
	}
	if record.GetCreateTimestampMs() <= 0 {
		return fmt.Errorf("create timestamp is invalid")
	}
	if record.GetSavedBaseVitality() == 0 || record.GetSavedBaseStrength() == 0 || record.GetSavedBaseToughness() == 0 || record.GetSavedBaseDexterity() == 0 {
		return fmt.Errorf("saved base attribute is incomplete")
	}
	if record.GetRawVitality() == 0 || record.GetRawStrength() == 0 || record.GetRawToughness() == 0 || record.GetRawDexterity() == 0 {
		return fmt.Errorf("raw attribute is incomplete")
	}
	return nil
}

func registerAccountRecordUUID(seen map[uint64]struct{}, usedUUID uint64, uuid uint64) error {
	if uuid == 0 {
		return fmt.Errorf("uuid is empty")
	}
	if uuid > usedUUID {
		return fmt.Errorf("uuid %d exceeds used uuid %d", uuid, usedUUID)
	}
	if _, exists := seen[uuid]; exists {
		return fmt.Errorf("uuid %d is duplicated", uuid)
	}
	seen[uuid] = struct{}{}
	return nil
}

func assetIDInRange(assetID uint64, start pb.AssetIDRange, end pb.AssetIDRange) bool {
	return assetID >= uint64(start) && assetID <= uint64(end)
}
