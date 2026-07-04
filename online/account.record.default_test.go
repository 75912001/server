package main

import (
	"errors"
	"math/rand"
	"testing"

	"server/common/gameconfig"
	pb "server/proto/pb"
)

func TestNormalizeCharacterAttribute(t *testing.T) {
	tests := []struct {
		name      string
		attribute *pb.CharacterAttributePoints
		want      characterAttributePoints
		wantErr   error
	}{
		{
			name:    "missing",
			wantErr: errCharacterAttributeInvalid,
		},
		{
			name: "total_not_20",
			attribute: &pb.CharacterAttributePoints{
				Vitality:  5,
				Strength:  5,
				Toughness: 5,
				Dexterity: 4,
			},
			wantErr: errCharacterAttributeInvalid,
		},
		{
			name: "single_value_over_20",
			attribute: &pb.CharacterAttributePoints{
				Vitality:  21,
				Strength:  0,
				Toughness: 0,
				Dexterity: 0,
			},
			wantErr: errCharacterAttributeInvalid,
		},
		{
			name: "valid",
			attribute: &pb.CharacterAttributePoints{
				Vitality:  8,
				Strength:  7,
				Toughness: 3,
				Dexterity: 2,
			},
			want: characterAttributePoints{
				vitality:  8,
				strength:  7,
				toughness: 3,
				dexterity: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCharacterAttribute(tt.attribute)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("normalizeCharacterAttribute() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got != tt.want {
				t.Fatalf("normalizeCharacterAttribute() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDefaultCharacterAssetIDRecordMapStoresRemainingRecordFields(t *testing.T) {
	record := defaultCharacterAssetIDRecordMap()

	assertRecordValue(t, record, pb.AssetIDRecord_AssetIDRecord_Direction, defaultCharacterDirection)
	assertRecordValue(t, record, pb.AssetIDRecord_AssetIDRecord_Action, defaultCharacterAction)
	if got := len(record); got != 2 {
		t.Fatalf("len(record) = %d, want 2", got)
	}
}

func TestInitializeDefaultAccountRecordCarriesFiveDefaultPets(t *testing.T) {
	loadGameConfigForDefaultRecordTest(t)

	record := &pb.AccountRecord{
		Aid:                      1,
		Account:                  "account",
		AccountCreateTimestampMs: 100,
	}
	err := initializeDefaultAccountRecord(
		record,
		0,
		defaultCharacterID,
		"角色",
		&pb.ElementalPoints{Earth: 10},
		&pb.CharacterAttributePoints{Vitality: 5, Strength: 5, Toughness: 5, Dexterity: 5},
		200,
	)
	if err != nil {
		t.Fatalf("initializeDefaultAccountRecord() error = %v", err)
	}
	if len(record.GetCharacterRecordList()) != 1 {
		t.Fatalf("len(record.CharacterRecordList) = %d, want 1", len(record.GetCharacterRecordList()))
	}
	if len(record.GetPetWarehouseRecordMap()) != 0 {
		t.Fatalf("len(record.PetWarehouseRecordMap) = %d, want 0", len(record.GetPetWarehouseRecordMap()))
	}

	character := record.GetCharacterRecordList()[0]
	if got := character.GetExp(); got != 0 {
		t.Fatalf("character.Exp = %d, want 0", got)
	}
	if got := character.GetElemental(); got.GetEarth() != 10 || got.GetWater() != 0 || got.GetFire() != 0 || got.GetWind() != 0 {
		t.Fatalf("character.Elemental = %+v, want earth:10", got)
	}
	if got := character.GetAvailablePoint(); got != defaultCharacterAvailablePoint {
		t.Fatalf("character.AvailablePoint = %d, want %d", got, defaultCharacterAvailablePoint)
	}
	if got := character.GetAttribute(); got.GetVitality() != 5 || got.GetStrength() != 5 || got.GetToughness() != 5 || got.GetDexterity() != 5 {
		t.Fatalf("character.Attribute = %+v, want all 5", got)
	}
	if got := character.GetCreateTimestampMs(); got != 200 {
		t.Fatalf("character.CreateTimestampMs = %d, want 200", got)
	}
	if got := character.GetRebirthCount(); got != 0 {
		t.Fatalf("character.RebirthCount = %d, want 0", got)
	}
	if got := character.GetSceneId(); got != defaultCharacterSceneID {
		t.Fatalf("character.SceneId = %d, want %d", got, defaultCharacterSceneID)
	}
	pets := character.GetPetRecordList()
	wantAssetIDs := []uint64{4000101, 4000102, 4000103, 4000104, 4000105}
	if len(pets) != len(wantAssetIDs) {
		t.Fatalf("len(character.PetRecordList) = %d, want %d", len(pets), len(wantAssetIDs))
	}
	for index, pet := range pets {
		if pet == nil {
			t.Fatalf("character.PetRecordList[%d] is nil", index)
		}
		if got := pet.GetExp(); got != 0 {
			t.Fatalf("character.PetRecordList[%d].Exp = %d, want 0", index, got)
		}
		if got := pet.GetLoyalty(); got != defaultPetLoyalty {
			t.Fatalf("character.PetRecordList[%d].Loyalty = %d, want %d", index, got, defaultPetLoyalty)
		}
		if got := pet.GetCreateTimestampMs(); got != 200 {
			t.Fatalf("character.PetRecordList[%d].CreateTimestampMs = %d, want 200", index, got)
		}
		if got := pet.GetRebirthCount(); got != 0 {
			t.Fatalf("character.PetRecordList[%d].RebirthCount = %d, want 0", index, got)
		}
		if pet.GetSavedBaseVitality() == 0 || pet.GetSavedBaseStrength() == 0 || pet.GetSavedBaseToughness() == 0 || pet.GetSavedBaseDexterity() == 0 {
			t.Fatalf("character.PetRecordList[%d] saved base is incomplete", index)
		}
		if pet.GetRawVitality() == 0 || pet.GetRawStrength() == 0 || pet.GetRawToughness() == 0 || pet.GetRawDexterity() == 0 {
			t.Fatalf("character.PetRecordList[%d] raw attribute is incomplete", index)
		}
		if got := len(pet.GetAssetRecordBaseMap()); got != 1 {
			t.Fatalf("len(character.PetRecordList[%d].AssetRecordBaseMap) = %d, want 1", index, got)
		}
		assertRecordValue(t, pet.GetAssetRecordBaseMap(), pb.AssetIDRecord_AssetIDRecord_AssetID, wantAssetIDs[index])
		wantStatus := pb.PetCarryStatus_PetCarryStatus_Wait
		if index == 0 {
			wantStatus = pb.PetCarryStatus_PetCarryStatus_Battle
		}
		if got := pet.GetCarryStatus(); got != wantStatus {
			t.Fatalf("character.PetRecordList[%d].CarryStatus = %s, want %s", index, got.String(), wantStatus.String())
		}
		if got := pet.GetGrade(); got != defaultPetGrade {
			t.Fatalf("character.PetRecordList[%d].Grade = %s, want %s", index, got.String(), defaultPetGrade.String())
		}
	}
}

func TestPetGradeSavedBaseOffset(t *testing.T) {
	tests := []struct {
		grade      pb.PetGrade
		wantOffset int
		wantErr    bool
	}{
		{grade: pb.PetGrade_PetGrade_Common, wantOffset: -2},
		{grade: pb.PetGrade_PetGrade_Rare, wantOffset: -1},
		{grade: pb.PetGrade_PetGrade_Epic, wantOffset: 0},
		{grade: pb.PetGrade_PetGrade_Legendary, wantOffset: 1},
		{grade: pb.PetGrade_PetGrade_Mythic, wantOffset: 2},
		{grade: pb.PetGrade_PetGrade_Unknow, wantErr: true},
		{grade: pb.PetGrade_PetGrade_Max, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.grade.String(), func(t *testing.T) {
			got, err := petGradeSavedBaseOffset(tt.grade)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("petGradeSavedBaseOffset(%s) error = nil, want error", tt.grade.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("petGradeSavedBaseOffset(%s) error = %v", tt.grade.String(), err)
			}
			if got != tt.wantOffset {
				t.Fatalf("petGradeSavedBaseOffset(%s) = %d, want %d", tt.grade.String(), got, tt.wantOffset)
			}
		})
	}
}

func TestCreatePetRuntimeAttributeUsesPetGradeOffset(t *testing.T) {
	loadGameConfigForDefaultRecordTest(t)

	const petID uint32 = 4000101
	pet := GGameConfig.Pet.GetByID(int(petID))
	if pet == nil {
		t.Fatalf("pet config %d not found", petID)
	}

	seed := int64(123)
	attr, err := createPetRuntimeAttribute(petID, 1, pb.PetGrade_PetGrade_Mythic, rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatalf("createPetRuntimeAttribute() error = %v", err)
	}
	assertPetSavedBase(t, attr, pet.Growth.BaseVital+2, pet.Growth.BaseStr+2, pet.Growth.BaseTough+2, pet.Growth.BaseDex+2)
	assertPetRawFromSavedBase(t, attr, pet.Growth.InitNum, randomFourPointDistribution(rand.New(rand.NewSource(seed))))

	attr, err = createPetRuntimeAttribute(petID, 1, pb.PetGrade_PetGrade_Common, rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatalf("createPetRuntimeAttribute() error = %v", err)
	}
	assertPetSavedBase(t, attr, pet.Growth.BaseVital-2, pet.Growth.BaseStr-2, pet.Growth.BaseTough-2, pet.Growth.BaseDex-2)
	assertPetRawFromSavedBase(t, attr, pet.Growth.InitNum, randomFourPointDistribution(rand.New(rand.NewSource(seed))))
}

func TestNewDefaultPetRecordInvalidGradeDoesNotConsumeUUID(t *testing.T) {
	loadGameConfigForDefaultRecordTest(t)

	record := &pb.AccountRecord{UsedUuid: 10}
	_, err := newDefaultPetRecord(record, 4000101, "pet", 0, pb.PetGrade_PetGrade_Unknow, pb.PetCarryStatus_PetCarryStatus_Wait, 123, rand.New(rand.NewSource(1)))
	if err == nil {
		t.Fatalf("newDefaultPetRecord() error = nil, want error")
	}
	if got := record.GetUsedUuid(); got != 10 {
		t.Fatalf("record.UsedUuid = %d, want 10", got)
	}
}

func TestSetCharacterLastLoginTimestamp(t *testing.T) {
	character := &pb.CharacterRecord{}

	rollback := setCharacterLastLoginTimestamp(character, 12345)

	if got := character.GetLastLoginTimestampMs(); got != 12345 {
		t.Fatalf("character.LastLoginTimestampMs = %d, want 12345", got)
	}
	rollback()
	if got := character.GetLastLoginTimestampMs(); got != 0 {
		t.Fatalf("character.LastLoginTimestampMs = %d, want 0 after rollback", got)
	}
}

func TestSetCharacterLastLoginTimestampRollbackRestoresPreviousValue(t *testing.T) {
	character := &pb.CharacterRecord{
		LastLoginTimestampMs: 100,
	}

	rollback := setCharacterLastLoginTimestamp(character, 200)

	if got := character.GetLastLoginTimestampMs(); got != 200 {
		t.Fatalf("character.LastLoginTimestampMs = %d, want 200", got)
	}
	rollback()
	if got := character.GetLastLoginTimestampMs(); got != 100 {
		t.Fatalf("character.LastLoginTimestampMs = %d, want 100", got)
	}
}

func TestSetCharacterLastLogoutTimestamp(t *testing.T) {
	character := &pb.CharacterRecord{}

	rollback := setCharacterLastLogoutTimestamp(character, 12345)

	if got := character.GetLastLogoutTimestampMs(); got != 12345 {
		t.Fatalf("character.LastLogoutTimestampMs = %d, want 12345", got)
	}
	rollback()
	if got := character.GetLastLogoutTimestampMs(); got != 0 {
		t.Fatalf("character.LastLogoutTimestampMs = %d, want 0 after rollback", got)
	}
}

func TestSetCharacterLastLogoutTimestampRollbackRestoresPreviousValue(t *testing.T) {
	character := &pb.CharacterRecord{
		LastLogoutTimestampMs: 100,
	}

	rollback := setCharacterLastLogoutTimestamp(character, 200)

	if got := character.GetLastLogoutTimestampMs(); got != 200 {
		t.Fatalf("character.LastLogoutTimestampMs = %d, want 200", got)
	}
	rollback()
	if got := character.GetLastLogoutTimestampMs(); got != 100 {
		t.Fatalf("character.LastLogoutTimestampMs = %d, want 100", got)
	}
}

func TestSetCharacterSceneIDRollbackRestoresPreviousValue(t *testing.T) {
	character := &pb.CharacterRecord{
		SceneId: 2000001,
	}

	rollback := setCharacterSceneID(character, 2000002)

	if got := character.GetSceneId(); got != 2000002 {
		t.Fatalf("character.SceneId = %d, want 2000002", got)
	}
	rollback()
	if got := character.GetSceneId(); got != 2000001 {
		t.Fatalf("character.SceneId = %d, want 2000001", got)
	}
}

func TestCharacterSceneIDRequiresValidSceneConfig(t *testing.T) {
	loadGameConfigForDefaultRecordTest(t)
	account := newTestAccountWithCharacter(1, 2000001, true)

	sceneID, err := account.characterSceneID(account.accountRecord.GetCharacterRecordList()[0])
	if err != nil {
		t.Fatalf("characterSceneID() error = %v", err)
	}
	if sceneID != 2000001 {
		t.Fatalf("characterSceneID() = %d, want 2000001", sceneID)
	}

	account.accountRecord.CharacterRecordList[0].SceneId = 2999999
	if _, err := account.characterSceneID(account.accountRecord.GetCharacterRecordList()[0]); err == nil {
		t.Fatalf("characterSceneID() error = nil, want error")
	}
}

func TestCurrentBattleCharacterAndPetUsesActiveCharacter(t *testing.T) {
	account := newTestAccountWithCharacter(1, 2000001, true)
	inactive := &pb.CharacterRecord{
		Uuid:      2,
		AssetId:   uint64(defaultCharacterID),
		SceneId:   2000001,
		Attribute: &pb.CharacterAttributePoints{Vitality: 5, Strength: 5, Toughness: 5, Dexterity: 5},
		PetRecordList: []*pb.PetRecord{
			{Uuid: 20, CarryStatus: pb.PetCarryStatus_PetCarryStatus_Battle},
		},
	}
	account.accountRecord.CharacterRecordList = append(account.accountRecord.CharacterRecordList, inactive)
	account.onlineCharacterUUIDSet[2] = struct{}{}

	character, pet, err := account.currentBattleCharacterAndPet()
	if err != nil {
		t.Fatalf("currentBattleCharacterAndPet() error = %v", err)
	}
	if character.GetUuid() != 1 || pet.GetUuid() != 10 {
		t.Fatalf("currentBattleCharacterAndPet() = character:%d pet:%d, want character:1 pet:10", character.GetUuid(), pet.GetUuid())
	}
}

func TestNewPlayerCharacterUnitUsesCharacterDirectFields(t *testing.T) {
	account := &Account{aid: 1}
	character := &pb.CharacterRecord{
		Uuid:    10,
		AssetId: uint64(defaultCharacterID),
		Exp:     1234,
		Attribute: &pb.CharacterAttributePoints{
			Vitality:  8,
			Strength:  7,
			Toughness: 3,
			Dexterity: 2,
		},
	}

	unit, err := account.newPlayerCharacterUnit(character)
	if err != nil {
		t.Fatalf("newPlayerCharacterUnit() error = %v", err)
	}
	if got := unit.GetAttribute().GetExp(); got != character.GetExp() {
		t.Fatalf("unit.Attribute.Exp = %d, want %d", got, character.GetExp())
	}
	if got := unit.GetAttribute().GetHp(); got != characterRuntimeHP(8, 7, 3, 2) {
		t.Fatalf("unit.Attribute.Hp = %d, want %d", got, characterRuntimeHP(8, 7, 3, 2))
	}
}

func TestNewPlayerPetUnitUsesPetDirectFields(t *testing.T) {
	account := &Account{aid: 1}
	character := &pb.CharacterRecord{Uuid: 10, AssetId: uint64(defaultCharacterID)}
	pet := &pb.PetRecord{
		Uuid:         20,
		Exp:          3456,
		Loyalty:      88,
		RawVitality:  10,
		RawStrength:  20,
		RawToughness: 30,
		RawDexterity: 40,
		AssetRecordBaseMap: map[uint32]uint64{
			uint32(pb.AssetIDRecord_AssetIDRecord_AssetID): 4000101,
		},
	}

	unit, err := account.newPlayerPetUnit(character, pet)
	if err != nil {
		t.Fatalf("newPlayerPetUnit() error = %v", err)
	}
	if got := unit.GetAttribute().GetExp(); got != pet.GetExp() {
		t.Fatalf("unit.Attribute.Exp = %d, want %d", got, pet.GetExp())
	}
	if got := unit.GetAttribute().GetLoyalty(); got != 88 {
		t.Fatalf("unit.Attribute.Loyalty = %d, want 88", got)
	}
}

func TestRandomSceneEnemyGroupUsesCurrentSceneConfig(t *testing.T) {
	loadGameConfigForDefaultRecordTest(t)
	account := newTestAccountWithCharacter(1, 2000002, true)

	group, err := account.randomSceneEnemyGroup(2000002)
	if err != nil {
		t.Fatalf("randomSceneEnemyGroup() error = %v", err)
	}
	if group.ID != 2 {
		t.Fatalf("randomSceneEnemyGroup() = %d, want 2", group.ID)
	}
}

func TestChooseWeightedSceneEnemyGroup(t *testing.T) {
	account := &Account{rng: rand.New(rand.NewSource(1))}
	if _, ok := account.chooseWeightedSceneEnemyGroup(nil); ok {
		t.Fatalf("chooseWeightedSceneEnemyGroup(nil) ok = true, want false")
	}
	group, ok := account.chooseWeightedSceneEnemyGroup([]gameconfig.SceneEnemyGroupEntry{
		{ID: 1, Weight: 0},
		{ID: 2, Weight: 100},
	})
	if !ok {
		t.Fatalf("chooseWeightedSceneEnemyGroup() ok = false, want true")
	}
	if group.ID != 2 {
		t.Fatalf("chooseWeightedSceneEnemyGroup() = %d, want 2", group.ID)
	}
}

func assertRecordValue(t *testing.T, record map[uint32]uint64, key pb.AssetIDRecord, want uint64) {
	t.Helper()
	if got := record[uint32(key)]; got != want {
		t.Fatalf("record[%s] = %d, want %d", key.String(), got, want)
	}
}

func assertRecordMissing(t *testing.T, record map[uint32]uint64, key pb.AssetIDRecord) {
	t.Helper()
	if got, ok := record[uint32(key)]; ok {
		t.Fatalf("record[%s] = %d, want missing", key.String(), got)
	}
}

func newTestAccountWithCharacter(characterUUID uint64, sceneID uint32, online bool) *Account {
	character := &pb.CharacterRecord{
		Uuid:      characterUUID,
		AssetId:   uint64(defaultCharacterID),
		SceneId:   sceneID,
		Attribute: &pb.CharacterAttributePoints{Vitality: 5, Strength: 5, Toughness: 5, Dexterity: 5},
		PetRecordList: []*pb.PetRecord{
			{Uuid: 10, CarryStatus: pb.PetCarryStatus_PetCarryStatus_Battle},
		},
	}
	account := &Account{
		aid: 1,
		accountRecord: &pb.AccountRecord{
			Aid:                            1,
			Account:                        "account",
			AccountCreateTimestampMs:       1,
			AccountRecordCreateTimestampMs: 1,
			CharacterRecordList:            []*pb.CharacterRecord{character},
		},
		onlineCharacterUUIDSet: map[uint64]struct{}{},
		rng:                    rand.New(rand.NewSource(1)),
	}
	if online {
		account.onlineCharacterUUIDSet[characterUUID] = struct{}{}
		account.activeCharacterUUID = characterUUID
	}
	return account
}

func assertPetSavedBase(t *testing.T, attr *petRuntimeAttribute, wantVital int, wantStr int, wantTough int, wantDex int) {
	t.Helper()
	if attr == nil {
		t.Fatalf("petRuntimeAttribute is nil")
	}
	if got := attr.savedBaseVital; got != uint64(wantVital) {
		t.Fatalf("savedBaseVital = %d, want %d", got, wantVital)
	}
	if got := attr.savedBaseStr; got != uint64(wantStr) {
		t.Fatalf("savedBaseStr = %d, want %d", got, wantStr)
	}
	if got := attr.savedBaseTough; got != uint64(wantTough) {
		t.Fatalf("savedBaseTough = %d, want %d", got, wantTough)
	}
	if got := attr.savedBaseDex; got != uint64(wantDex) {
		t.Fatalf("savedBaseDex = %d, want %d", got, wantDex)
	}
}

func assertPetRawFromSavedBase(t *testing.T, attr *petRuntimeAttribute, initFactor int, bonus fourPointDistribution) {
	t.Helper()
	if attr == nil {
		t.Fatalf("petRuntimeAttribute is nil")
	}
	wantRawVital := uint64(float64(initFactor) * float64(int(attr.savedBaseVital)+bonus.vital))
	wantRawStr := uint64(float64(initFactor) * float64(int(attr.savedBaseStr)+bonus.str))
	wantRawTough := uint64(float64(initFactor) * float64(int(attr.savedBaseTough)+bonus.tough))
	wantRawDex := uint64(float64(initFactor) * float64(int(attr.savedBaseDex)+bonus.dex))
	if attr.rawVital != wantRawVital || attr.rawStr != wantRawStr || attr.rawTough != wantRawTough || attr.rawDex != wantRawDex {
		t.Fatalf("raw = (%d,%d,%d,%d), want (%d,%d,%d,%d)", attr.rawVital, attr.rawStr, attr.rawTough, attr.rawDex, wantRawVital, wantRawStr, wantRawTough, wantRawDex)
	}
}

func loadGameConfigForDefaultRecordTest(t *testing.T) {
	t.Helper()
	previous := GGameConfig
	manager, err := gameconfig.Load("../config")
	if err != nil {
		t.Fatalf("gameconfig.Load() error = %v", err)
	}
	GGameConfig = manager
	t.Cleanup(func() {
		GGameConfig = previous
	})
}
