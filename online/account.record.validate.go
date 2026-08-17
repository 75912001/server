package main

import (
	"fmt"
	"math"
	"strings"

	"server/common/gameconfig"
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
	for itemID := range record.GetItemWarehouse().GetItemCountMap() {
		if isCharacterAssetItemID(itemID) {
			return fmt.Errorf("item warehouse contains character asset %d", itemID)
		}
	}
	if err := validateEquipmentContainer(record.GetItemWarehouse(), int(pb.AccountRecordLimit_AccountRecordLimit_MaxItemWarehouseCount), seenUUID, usedUUID); err != nil {
		return fmt.Errorf("item warehouse: %w", err)
	}

	for slot, characterRecord := range record.GetCharacterRecordList() {
		if characterRecord == nil {
			return fmt.Errorf("character slot %d is nil", slot)
		}
		characterUUID := characterRecord.GetBase().GetUuid()
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
	if record.GetBase() == nil {
		return fmt.Errorf("base is nil")
	}
	base := record.GetBase()
	if strings.TrimSpace(base.GetNick()) == "" {
		return fmt.Errorf("nick is empty")
	}
	if !assetIDInRange(base.GetAssetId(), pb.AssetIDRange_AssetIDRange_Character_Start, pb.AssetIDRange_AssetIDRange_Character_End) {
		return fmt.Errorf("asset id %d is invalid", base.GetAssetId())
	}
	if base.GetCreateTimestampMs() <= 0 {
		return fmt.Errorf("create timestamp is invalid")
	}
	if !assetIDInRange(uint64(base.GetSceneId()), pb.AssetIDRange_AssetIDRange_Scene_Start, pb.AssetIDRange_AssetIDRange_Scene_End) {
		return fmt.Errorf("scene id %d is invalid", base.GetSceneId())
	}
	if base.GetVitality()+base.GetStrength()+base.GetToughness()+base.GetDexterity() == 0 {
		return fmt.Errorf("attribute is empty")
	}
	if base.GetDuelPoint() > uint32(pb.CharacterLimit_CharacterLimit_MaxDuelPoint) {
		return fmt.Errorf("duel point %d exceeds limit", base.GetDuelPoint())
	}
	if base.GetCharm() > characterMaxCharm {
		return fmt.Errorf("charm %d exceeds limit", base.GetCharm())
	}
	if err := validateCharacterLuckState(base.GetLuckState()); err != nil {
		return fmt.Errorf("luck state: %w", err)
	}
	if base.GetLuckState().GetBaseLuck() == 0 && (base.GetLastLoginTimestampMs() != 0 || base.GetLastLogoutTimestampMs() != 0) {
		return fmt.Errorf("pending luck state has login history")
	}
	if len(record.GetPetRecordList()) > int(pb.PetRecordLimit_PetRecordLimit_MaxCarryCount) {
		return fmt.Errorf("carried pet count exceeds limit")
	}
	for assetID, count := range record.GetAssetCountMap() {
		if !isCharacterAssetItemID(assetID) {
			return fmt.Errorf("character asset id %d is outside [%d,%d]", assetID, pb.AssetIDRange_AssetIDRange_CharacterAsset_Start, pb.AssetIDRange_AssetIDRange_CharacterAsset_End)
		}
		if count > uint64(math.MaxInt64) {
			return fmt.Errorf("character asset %d count %d exceeds max int64", assetID, count)
		}
		if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil || gameconfig.GGameConfig.Item.Get(assetID) == nil {
			return fmt.Errorf("character asset config %d is missing", assetID)
		}
	}
	for itemID := range record.GetItemBag().GetItemCountMap() {
		if isCharacterAssetItemID(itemID) {
			return fmt.Errorf("item bag contains character asset %d", itemID)
		}
	}
	if err := validateEquipmentContainer(record.GetItemBag(), int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount), seenUUID, usedUUID); err != nil {
		return fmt.Errorf("item bag: %w", err)
	}
	if err := validateCharacterEquipment(record, seenUUID, usedUUID); err != nil {
		return fmt.Errorf("equipped item: %w", err)
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
	if !assetIDInRange(uint64(record.GetAssetId()), pb.AssetIDRange_AssetIDRange_Pet_Start, pb.AssetIDRange_AssetIDRange_Pet_End) {
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
