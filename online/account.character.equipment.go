package main

import (
	"errors"
	"fmt"
	"math"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xutil "github.com/75912001/xlib/util"
	"google.golang.org/protobuf/proto"
)

var (
	errCharacterEquipmentInvalidArgument    = errors.New("invalid character equipment argument")
	errCharacterEquipmentTargetNotFound     = errors.New("character equipment target not found")
	errCharacterEquipmentFailedPrecondition = errors.New("character equipment precondition failed")
	errCharacterEquipmentResourceExhausted  = errors.New("character equipment resource exhausted")
	errCharacterEquipmentRecordInvalid      = errors.New("character equipment record is invalid")
)

var equipmentFixedModifierKeys = [...]pb.EquipmentRecordBase{
	pb.EquipmentRecordBase_EquipmentRecordBase_AttackModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_DefenceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_QuickModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_MaxHPModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_MaxMPModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_LuckModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_CharmModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_AvoidModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_PoisonResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_ParalysisResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_SleepResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_StoneResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_DrunkResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_ConfusionResistanceModifier,
	pb.EquipmentRecordBase_EquipmentRecordBase_CriticalModifier,
}

var supportedCharacterEquipmentTypes = [...]pb.EquipmentType{
	pb.EquipmentType_EquipmentType_Weapon,
	pb.EquipmentType_EquipmentType_Accessory1,
	pb.EquipmentType_EquipmentType_Accessory2,
}

func configuredEquipmentEntry(assetID uint32) (*gameconfig.ItemEntry, error) {
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil {
		return nil, fmt.Errorf("item config is not loaded")
	}
	entry := gameconfig.GGameConfig.Item.Get(assetID)
	if entry == nil || entry.ID == nil || *entry.ID != assetID {
		return nil, fmt.Errorf("equipment config %d is missing or mismatched", assetID)
	}
	if assetID >= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Start) &&
		assetID <= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_End) {
		if entry.WeaponType <= pb.CharacterWeaponType_CharacterWeaponType_Unknow ||
			entry.WeaponType >= pb.CharacterWeaponType_CharacterWeaponType_Max || entry.AccessoryType != pb.AccessoryType_AccessoryType_Unknow {
			return nil, fmt.Errorf("weapon config %d type is invalid", assetID)
		}
	} else {
		minimum, maximum := gameconfig.AccessoryIDRange(entry.AccessoryType)
		if minimum == 0 || assetID < minimum || assetID > maximum || entry.WeaponType != pb.CharacterWeaponType_CharacterWeaponType_Unknow {
			return nil, fmt.Errorf("accessory config %d type %d does not match its id range", assetID, entry.AccessoryType)
		}
	}
	return entry, nil
}

func configuredWeaponEntry(assetID uint32) (*gameconfig.ItemEntry, error) {
	entry, err := configuredEquipmentEntry(assetID)
	if err != nil {
		return nil, err
	}
	if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unknow {
		return nil, fmt.Errorf("equipment %d is not a weapon", assetID)
	}
	return entry, nil
}

// 仅开放的部位返回字段指针, 使穿戴、卸下和响应始终访问同一个目标字段.
func characterEquipmentSlot(equipment *pb.CharacterEquipmentRecord, equipmentType pb.EquipmentType) **pb.EquipmentRecord {
	if equipment == nil {
		return nil
	}
	switch equipmentType {
	case pb.EquipmentType_EquipmentType_Weapon:
		return &equipment.Weapon
	case pb.EquipmentType_EquipmentType_Accessory1:
		return &equipment.Accessory1
	case pb.EquipmentType_EquipmentType_Accessory2:
		return &equipment.Accessory2
	default:
		return nil
	}
}

func equipmentModifierRange(entry *gameconfig.ItemEntry, key pb.EquipmentRecordBase) (int32, int32, bool) {
	if entry == nil {
		return 0, 0, false
	}
	switch key {
	case pb.EquipmentRecordBase_EquipmentRecordBase_AttackModifier:
		return entry.AttackMin, entry.AttackMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_DefenceModifier:
		return entry.DefenceMin, entry.DefenceMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_QuickModifier:
		return entry.QuickMin, entry.QuickMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_MaxHPModifier:
		return entry.HPMin, entry.HPMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_MaxMPModifier:
		return entry.MPMin, entry.MPMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_LuckModifier:
		return entry.LuckMin, entry.LuckMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_CharmModifier:
		return entry.CharmMin, entry.CharmMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_AvoidModifier:
		return entry.AvoidMin, entry.AvoidMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_PoisonResistanceModifier:
		return entry.PoisonMin, entry.PoisonMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_ParalysisResistanceModifier:
		return entry.ParalysisMin, entry.ParalysisMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_SleepResistanceModifier:
		return entry.SleepMin, entry.SleepMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_StoneResistanceModifier:
		return entry.StoneMin, entry.StoneMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_DrunkResistanceModifier:
		return entry.DrunkMin, entry.DrunkMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_ConfusionResistanceModifier:
		return entry.ConfusionMin, entry.ConfusionMax, true
	case pb.EquipmentRecordBase_EquipmentRecordBase_CriticalModifier:
		return entry.CriticalMin, entry.CriticalMax, true
	default:
		return 0, 0, false
	}
}

// newEquipmentRecord 按原版创建时机把15项[min,max]独立随机一次并完整固化到实例.
func newEquipmentRecord(equipmentUUID uint64, assetID uint32) (*pb.EquipmentRecord, error) {
	if equipmentUUID == 0 {
		return nil, fmt.Errorf("equipment uuid is empty")
	}
	entry, err := configuredEquipmentEntry(assetID)
	if err != nil {
		return nil, err
	}
	recordBaseMap := make(map[int32]int64, len(equipmentFixedModifierKeys))
	for _, key := range equipmentFixedModifierKeys {
		minimum, maximum, ok := equipmentModifierRange(entry, key)
		if !ok || minimum > maximum {
			return nil, fmt.Errorf("equipment %d modifier %s range is invalid", assetID, key)
		}
		width := uint64(int64(maximum) - int64(minimum))
		value := int64(minimum) + int64(xutil.RandomU64(0, width))
		recordBaseMap[int32(key)] = value
	}
	return &pb.EquipmentRecord{
		Uuid:          equipmentUUID,
		AssetId:       assetID,
		RecordBaseMap: recordBaseMap,
	}, nil
}

// validateEquipmentRecord 不迁移旧实例: 15项必须齐全且无额外key, 固化值必须仍在当前配置范围内.
func validateEquipmentRecord(record *pb.EquipmentRecord, expectedUUID uint64) error {
	if record == nil || expectedUUID == 0 || record.GetUuid() != expectedUUID {
		return fmt.Errorf("equipment key %d does not match record uuid %d", expectedUUID, record.GetUuid())
	}
	entry, err := configuredEquipmentEntry(record.GetAssetId())
	if err != nil {
		return err
	}
	if len(record.GetRecordBaseMap()) != len(equipmentFixedModifierKeys) {
		return fmt.Errorf("equipment %d fixed modifier count %d is not %d", expectedUUID, len(record.GetRecordBaseMap()), len(equipmentFixedModifierKeys))
	}
	for _, key := range equipmentFixedModifierKeys {
		value, exists := record.GetRecordBaseMap()[int32(key)]
		if !exists {
			return fmt.Errorf("equipment %d fixed modifier %s is missing", expectedUUID, key)
		}
		minimum, maximum, ok := equipmentModifierRange(entry, key)
		if !ok || value < int64(minimum) || value > int64(maximum) {
			return fmt.Errorf("equipment %d fixed modifier %s value %d is outside [%d,%d]", expectedUUID, key, value, minimum, maximum)
		}
	}
	return nil
}

func validateEquipmentContainer(container *pb.ItemContainerRecord, capacity int, seenUUID map[uint64]struct{}, usedUUID uint64) error {
	if container == nil {
		return fmt.Errorf("item container is nil")
	}
	if itemContainerCount(container) > capacity {
		return fmt.Errorf("item container count %d exceeds %d", itemContainerCount(container), capacity)
	}
	for equipmentUUID, record := range container.GetEquipmentRecordMap() {
		if err := registerAccountRecordUUID(seenUUID, usedUUID, equipmentUUID); err != nil {
			return err
		}
		if err := validateEquipmentRecord(record, equipmentUUID); err != nil {
			return err
		}
	}
	return nil
}

func validateCharacterEquipment(record *pb.CharacterRecord, seenUUID map[uint64]struct{}, usedUUID uint64) error {
	if record == nil {
		return fmt.Errorf("character record is nil")
	}
	if err := validateCharacterEquipmentSlots(record.GetEquipment()); err != nil {
		return err
	}
	for _, equipmentType := range supportedCharacterEquipmentTypes {
		if equipped := *characterEquipmentSlot(record.GetEquipment(), equipmentType); equipped != nil {
			if err := registerAccountRecordUUID(seenUUID, usedUUID, equipped.GetUuid()); err != nil {
				return err
			}
		}
	}
	return nil
}

// 原版两个首饰位共享六类首饰, 禁止同类同时穿戴; 不能只比较实例UUID或道具ID.
func validateCharacterEquipmentSlots(equipment *pb.CharacterEquipmentRecord) error {
	if equipment == nil {
		return fmt.Errorf("character equipment is nil")
	}
	unsupported := []struct {
		name   string
		record *pb.EquipmentRecord
	}{
		{name: "helmet", record: equipment.GetHelmet()},
		{name: "chest", record: equipment.GetChest()},
		{name: "shield", record: equipment.GetShield()},
		{name: "gloves", record: equipment.GetGloves()},
		{name: "belt", record: equipment.GetBelt()},
		{name: "boots", record: equipment.GetBoots()},
	}
	for _, slot := range unsupported {
		if slot.record != nil {
			return fmt.Errorf("unsupported equipped slot %s is populated", slot.name)
		}
	}
	accessoryType := pb.AccessoryType_AccessoryType_Unknow
	for _, equipmentType := range supportedCharacterEquipmentTypes {
		equipped := *characterEquipmentSlot(equipment, equipmentType)
		if equipped == nil {
			continue
		}
		if err := validateEquipmentRecord(equipped, equipped.GetUuid()); err != nil {
			return err
		}
		entry := gameconfig.GGameConfig.Item.Get(equipped.GetAssetId())
		if equipmentType == pb.EquipmentType_EquipmentType_Weapon {
			if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unknow {
				return fmt.Errorf("weapon slot contains accessory %d", equipped.GetAssetId())
			}
		} else {
			if entry.AccessoryType == pb.AccessoryType_AccessoryType_Unknow {
				return fmt.Errorf("accessory slot contains weapon %d", equipped.GetAssetId())
			}
			if entry.AccessoryType == accessoryType {
				return fmt.Errorf("cannot equip two accessories of type %s", accessoryType)
			}
			accessoryType = entry.AccessoryType
		}
	}
	return nil
}

func clampEquipmentValue(value int64, minimum int64, maximum int64) int64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func equipmentModifier(record *pb.EquipmentRecord, key pb.EquipmentRecordBase) int64 {
	if record == nil {
		return 0
	}
	return record.GetRecordBaseMap()[int32(key)]
}

// characterEffectiveAttribute 保留当前项目裸装公式, 再按原版ITEM_equipEffect顺序叠加实例固化值.
func characterEffectiveAttribute(record *pb.CharacterRecord) (*pb.CharacterEffectiveAttribute, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 || record.GetEquipment() == nil {
		return nil, fmt.Errorf("character record is incomplete")
	}
	if err := validateCharacterEquipmentSlots(record.GetEquipment()); err != nil {
		return nil, err
	}
	base := record.GetBase()
	vitality := int64(base.GetVitality())
	strength := int64(base.GetStrength())
	toughness := int64(base.GetToughness())
	dexterity := int64(base.GetDexterity())
	maxHP := vitality*4 + strength + toughness + dexterity
	attack := strength + toughness/10 + vitality/10 + dexterity/20
	defense := toughness + strength/10 + vitality/10 + dexterity/20
	agility := dexterity
	if attack < 1 {
		attack = 1
	}
	if defense < 1 {
		defense = 1
	}
	if agility < 1 {
		agility = 1
	}

	maxMP := int64(pb.CharacterLimit_CharacterLimit_MagicPointMax)
	luck := int64(base.GetLuckState().GetBaseLuck())
	charm := int64(base.GetCharm())
	avoid := int64(0)
	critical := int64(0)
	poison := int64(0)
	paralysis := int64(0)
	sleep := int64(0)
	stone := int64(0)
	drunk := int64(0)
	confusion := int64(0)
	otherDamage := int64(0)
	otherDefense := int64(0)
	elemental := [4]int64{int64(base.GetEarth()), int64(base.GetWater()), int64(base.GetFire()), int64(base.GetWind())}
	weaponType := pb.CharacterWeaponType_CharacterWeaponType_Unarmed

	for _, equipmentType := range supportedCharacterEquipmentTypes {
		equipped := *characterEquipmentSlot(record.GetEquipment(), equipmentType)
		if equipped == nil {
			continue
		}
		entry := gameconfig.GGameConfig.Item.Get(equipped.GetAssetId())
		if equipmentType == pb.EquipmentType_EquipmentType_Weapon {
			weaponType = entry.WeaponType
		}
		maxHP += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_MaxHPModifier)
		attack += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_AttackModifier)
		defense += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_DefenceModifier)
		agility += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_QuickModifier)
		maxMP += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_MaxMPModifier)
		luck += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_LuckModifier)
		charm += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_CharmModifier)
		avoid += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_AvoidModifier)
		poison += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_PoisonResistanceModifier)
		paralysis += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_ParalysisResistanceModifier)
		sleep += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_SleepResistanceModifier)
		stone += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_StoneResistanceModifier)
		drunk += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_DrunkResistanceModifier)
		confusion += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_ConfusionResistanceModifier)
		critical += equipmentModifier(equipped, pb.EquipmentRecordBase_EquipmentRecordBase_CriticalModifier)
		otherDamage += int64(entry.OtherDamage)
		otherDefense += int64(entry.OtherDefence)
		if entry.Attribute >= 1 && entry.Attribute <= 4 {
			selected := int(entry.Attribute - 1)
			for index := range elemental {
				if index == selected {
					elemental[index] += int64(entry.AttributeValue)
				} else {
					elemental[index] -= int64(entry.AttributeValue)
				}
			}
		}
	}

	for index := range elemental {
		elemental[index] = clampEquipmentValue(elemental[index], 0, 10)
	}
	return &pb.CharacterEffectiveAttribute{
		CharacterUuid:               base.GetUuid(),
		MaxHp:                       uint32(clampEquipmentValue(maxHP, 0, 10_000_000)),
		Attack:                      uint32(clampEquipmentValue(attack, 0, 10_000_000)),
		Defense:                     int32(clampEquipmentValue(defense, -100, 10_000_000)),
		Agility:                     int32(clampEquipmentValue(agility, -100, 10_000_000)),
		MaxMp:                       uint32(clampEquipmentValue(maxMP, 0, 1000)),
		EffectiveLuck:               uint32(clampEquipmentValue(luck, 1, 5)),
		EffectiveCharm:              uint32(clampEquipmentValue(charm, 0, 100)),
		EffectiveAvoid:              int32(clampEquipmentValue(avoid, 0, 10_000_000)),
		CriticalModifier:            int32(clampEquipmentValue(critical, math.MinInt32, math.MaxInt32)),
		PoisonResistanceModifier:    int32(clampEquipmentValue(poison, math.MinInt32, math.MaxInt32)),
		ParalysisResistanceModifier: int32(clampEquipmentValue(paralysis, math.MinInt32, math.MaxInt32)),
		SleepResistanceModifier:     int32(clampEquipmentValue(sleep, math.MinInt32, math.MaxInt32)),
		StoneResistanceModifier:     int32(clampEquipmentValue(stone, math.MinInt32, math.MaxInt32)),
		DrunkResistanceModifier:     int32(clampEquipmentValue(drunk, math.MinInt32, math.MaxInt32)),
		ConfusionResistanceModifier: int32(clampEquipmentValue(confusion, math.MinInt32, math.MaxInt32)),
		Elemental:                   &pb.ElementalPoints{Earth: uint32(elemental[0]), Water: uint32(elemental[1]), Fire: uint32(elemental[2]), Wind: uint32(elemental[3])},
		OtherDamageModifier:         int32(clampEquipmentValue(otherDamage, math.MinInt32, math.MaxInt32)),
		OtherDefenseModifier:        int32(clampEquipmentValue(otherDefense, math.MinInt32, math.MaxInt32)),
		WeaponType:                  weaponType,
	}, nil
}

func characterEffectiveAttributeList(record *pb.AccountRecord) ([]*pb.CharacterEffectiveAttribute, error) {
	if record == nil {
		return nil, fmt.Errorf("account record is nil")
	}
	result := make([]*pb.CharacterEffectiveAttribute, 0, len(record.GetCharacterRecordList()))
	for slot, characterRecord := range record.GetCharacterRecordList() {
		if characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 {
			continue
		}
		effective, err := characterEffectiveAttribute(characterRecord)
		if err != nil {
			return nil, fmt.Errorf("character slot %d effective attribute: %w", slot, err)
		}
		result = append(result, effective)
	}
	return result, nil
}

type characterEquipmentReplacePlan struct {
	characterUUID     uint64
	equipmentType     pb.EquipmentType
	characterSlot     int
	previousCharacter *pb.CharacterRecord
	nextCharacter     *pb.CharacterRecord
	nextAccountRecord *pb.AccountRecord
	effective         *pb.CharacterEffectiveAttribute
}

func prepareCharacterEquipmentReplacePlan(accountRecord *pb.AccountRecord, characterRecord *pb.CharacterRecord, equipmentType pb.EquipmentType, equipmentUUID uint64) (*characterEquipmentReplacePlan, error) {
	if accountRecord == nil || characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 || characterEquipmentSlot(characterRecord.GetEquipment(), equipmentType) == nil {
		return nil, errCharacterEquipmentInvalidArgument
	}
	characterSlot := -1
	for index, candidate := range accountRecord.GetCharacterRecordList() {
		if candidate == characterRecord && candidate.GetBase().GetUuid() == characterRecord.GetBase().GetUuid() {
			characterSlot = index
			break
		}
	}
	if characterSlot < 0 {
		return nil, fmt.Errorf("%w: character slot not found", errCharacterEquipmentRecordInvalid)
	}

	nextAccountRecord := proto.Clone(accountRecord).(*pb.AccountRecord)
	nextCharacter := nextAccountRecord.GetCharacterRecordList()[characterSlot]
	if nextCharacter.ItemBag == nil || nextCharacter.Equipment == nil {
		return nil, fmt.Errorf("%w: character equipment container is missing", errCharacterEquipmentRecordInvalid)
	}
	if nextCharacter.ItemBag.EquipmentRecordMap == nil {
		nextCharacter.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}
	targetSlot := characterEquipmentSlot(nextCharacter.Equipment, equipmentType)
	currentEquipment := *targetSlot
	if equipmentUUID == 0 {
		if currentEquipment == nil {
			return nil, fmt.Errorf("%w: equipment slot is already empty", errCharacterEquipmentFailedPrecondition)
		}
		if itemContainerCount(nextCharacter.GetItemBag()) >= int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
			return nil, errCharacterEquipmentResourceExhausted
		}
		if _, exists := nextCharacter.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: equipment %d already exists in bag", errCharacterEquipmentRecordInvalid, currentEquipment.GetUuid())
		}
		nextCharacter.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()] = currentEquipment
		*targetSlot = nil
	} else {
		nextEquipment := nextCharacter.ItemBag.GetEquipmentRecordMap()[equipmentUUID]
		if nextEquipment == nil {
			return nil, fmt.Errorf("%w: equipment %d is not in character bag", errCharacterEquipmentTargetNotFound, equipmentUUID)
		}
		if err := validateEquipmentRecord(nextEquipment, equipmentUUID); err != nil {
			return nil, fmt.Errorf("%w: %v", errCharacterEquipmentRecordInvalid, err)
		}
		entry, err := configuredEquipmentEntry(nextEquipment.GetAssetId())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errCharacterEquipmentTargetNotFound, err)
		}
		if equipmentType == pb.EquipmentType_EquipmentType_Weapon {
			if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unknow {
				return nil, fmt.Errorf("%w: accessory cannot enter weapon slot", errCharacterEquipmentFailedPrecondition)
			}
		} else {
			if entry.AccessoryType == pb.AccessoryType_AccessoryType_Unknow {
				return nil, fmt.Errorf("%w: weapon cannot enter accessory slot", errCharacterEquipmentFailedPrecondition)
			}
			otherAccessory := nextCharacter.Equipment.GetAccessory1()
			if equipmentType == pb.EquipmentType_EquipmentType_Accessory1 {
				otherAccessory = nextCharacter.Equipment.GetAccessory2()
			}
			if otherAccessory != nil {
				otherEntry, err := configuredEquipmentEntry(otherAccessory.GetAssetId())
				if err != nil {
					return nil, fmt.Errorf("%w: %v", errCharacterEquipmentRecordInvalid, err)
				}
				if entry.AccessoryType == otherEntry.AccessoryType {
					return nil, fmt.Errorf("%w: cannot equip two accessories of type %s", errCharacterEquipmentFailedPrecondition, entry.AccessoryType)
				}
			}
		}
		if gameconfig.GGameConfig.Exp == nil {
			return nil, fmt.Errorf("%w: exp config is not loaded", errCharacterEquipmentRecordInvalid)
		}
		characterLevel, err := gameconfig.GGameConfig.Exp.GetLevel(nextCharacter.GetBase().GetExp())
		if err != nil {
			return nil, fmt.Errorf("%w: character level: %v", errCharacterEquipmentRecordInvalid, err)
		}
		if characterLevel < entry.Level {
			return nil, fmt.Errorf("%w: character level %d is below equipment level %d", errCharacterEquipmentFailedPrecondition, characterLevel, entry.Level)
		}
		// 当前角色档案尚未接入转职状态, 因此角色权威职业为None, 不能装备带neprof限制的装备.
		if entry.Profession != pb.CharacterProfession_CharacterProfession_None {
			return nil, fmt.Errorf("%w: equipment requires profession %s", errCharacterEquipmentFailedPrecondition, entry.Profession)
		}
		delete(nextCharacter.ItemBag.EquipmentRecordMap, equipmentUUID)
		if currentEquipment != nil {
			if _, exists := nextCharacter.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()]; exists {
				return nil, fmt.Errorf("%w: current equipment %d already exists in bag", errCharacterEquipmentRecordInvalid, currentEquipment.GetUuid())
			}
			nextCharacter.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()] = currentEquipment
		}
		*targetSlot = nextEquipment
	}

	effective, err := characterEffectiveAttribute(nextCharacter)
	if err != nil {
		return nil, fmt.Errorf("%w: effective attribute: %v", errCharacterEquipmentRecordInvalid, err)
	}
	return &characterEquipmentReplacePlan{
		characterUUID:     characterRecord.GetBase().GetUuid(),
		equipmentType:     equipmentType,
		characterSlot:     characterSlot,
		previousCharacter: characterRecord,
		nextCharacter:     nextCharacter,
		nextAccountRecord: nextAccountRecord,
		effective:         effective,
	}, nil
}

func persistCharacterEquipmentReplacePlan(plan *characterEquipmentReplacePlan, accountRecord *pb.AccountRecord, character *character, persist func(*pb.AccountRecord) error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || plan.nextAccountRecord == nil || plan.nextCharacter == nil {
		return errCharacterEquipmentInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != plan.previousCharacter || character.record != plan.previousCharacter {
		return fmt.Errorf("%w: authoritative character changed before persistence", errCharacterEquipmentRecordInvalid)
	}
	if err := persist(plan.nextAccountRecord); err != nil {
		return err
	}
	accountRecord.CharacterRecordList[plan.characterSlot] = plan.nextCharacter
	character.record = plan.nextCharacter
	return nil
}

func (p *Account) onCharacterEquipmentReplaceReq(gateway *Gateway, packet *pb.OnlineClientPacket) {
	var request pb.CharacterEquipmentReplaceReq
	if err := proto.Unmarshal(packet.GetBody(), &request); err != nil || request.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(request.GetCharacterUuid())
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	plan, err := prepareCharacterEquipmentReplacePlan(p.accountRecord, character.record, request.GetEquipmentType(), request.GetEquipmentUuid())
	if err != nil {
		xlog.GLog.Warnf("character equipment replace rejected aid:%d character:%d type:%s equipment:%d err:%v", p.aid, request.GetCharacterUuid(), request.GetEquipmentType(), request.GetEquipmentUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), characterEquipmentResultID(err))
		return
	}
	if err := persistCharacterEquipmentReplacePlan(plan, p.accountRecord, character, func(next *pb.AccountRecord) error {
		return unaryCacheSetAccountRecord(p.aid, next)
	}); err != nil {
		xlog.GLog.Errorf("persist character equipment replace failed aid:%d character:%d type:%s equipment:%d err:%v", p.aid, request.GetCharacterUuid(), request.GetEquipmentType(), request.GetEquipmentUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.Internal.Code())
		return
	}
	// 响应只携带本次替换部位的装备; 卸下目标部位装备时该字段保持未设置.
	var replacedEquipment *pb.EquipmentRecord
	if equipped := *characterEquipmentSlot(plan.nextCharacter.GetEquipment(), plan.equipmentType); equipped != nil {
		replacedEquipment = proto.Clone(equipped).(*pb.EquipmentRecord)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.Success.Code(), &pb.CharacterEquipmentReplaceRes{
		CharacterUuid:      plan.characterUUID,
		EquipmentType:      plan.equipmentType,
		ItemBag:            proto.Clone(plan.nextCharacter.GetItemBag()).(*pb.ItemContainerRecord),
		Equipment:          replacedEquipment,
		EffectiveAttribute: proto.Clone(plan.effective).(*pb.CharacterEffectiveAttribute),
	})
}

func characterEquipmentResultID(err error) uint32 {
	switch {
	case errors.Is(err, errCharacterEquipmentInvalidArgument):
		return xerror.InvalidArgument.Code()
	case errors.Is(err, errCharacterEquipmentTargetNotFound):
		return xerror.NotFound.Code()
	case errors.Is(err, errCharacterEquipmentFailedPrecondition):
		return xerror.FailedPrecondition.Code()
	case errors.Is(err, errCharacterEquipmentResourceExhausted):
		return xerror.ResourceExhausted.Code()
	default:
		return xerror.Internal.Code()
	}
}

func equipmentFixedModifierValueInt32(record *pb.EquipmentRecord, key pb.EquipmentRecordBase) int32 {
	value := equipmentModifier(record, key)
	if value < math.MinInt32 {
		return math.MinInt32
	}
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(value)
}
