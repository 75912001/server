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

var equipmentBaseAttributeKeys = [...]pb.EquipmentRecordAttribute{
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Attack,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Defence,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Quick,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxHP,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxMP,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Luck,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Charm,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Avoid,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_PoisonResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ParalysisResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Critical,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Counter,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DamageBonusPercent,
	pb.EquipmentRecordAttribute_EquipmentRecordAttribute_CritDamageBonusPercent,
}

var supportedCharacterEquipmentTypes = [...]pb.EquipmentType{
	pb.EquipmentType_EquipmentType_Weapon,
	pb.EquipmentType_EquipmentType_Chest,
	pb.EquipmentType_EquipmentType_Helmet,
	pb.EquipmentType_EquipmentType_Shield,
	pb.EquipmentType_EquipmentType_Belt,
	pb.EquipmentType_EquipmentType_Boots,
	pb.EquipmentType_EquipmentType_Accessory1,
	pb.EquipmentType_EquipmentType_Accessory2,
}

func isChestEquipmentAssetID(assetID uint32) bool {
	return assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_End)
}

func isHelmetEquipmentAssetID(assetID uint32) bool {
	return assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_End)
}

func isShieldEquipmentAssetID(assetID uint32) bool {
	return assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_End)
}

func isBeltEquipmentAssetID(assetID uint32) bool {
	return assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_End)
}

func isBootsEquipmentAssetID(assetID uint32) bool {
	return assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_End)
}

// isArmorEquipmentAssetID只接受当前协议已分配的六类普通防具区间.
// 防具可以作为装备实例进入背包, 是否开放对应穿戴槽由换装链路独立判断.
func isArmorEquipmentAssetID(assetID uint32) bool {
	switch {
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_End):
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_End):
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_End):
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Gloves_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Gloves_End):
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_End):
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_End):
	default:
		return false
	}
	return true
}

func configuredEquipmentEntry(assetID uint32) (*gameconfig.ItemEntry, error) {
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil {
		return nil, fmt.Errorf("item config is not loaded")
	}
	entry := gameconfig.GGameConfig.Item.Get(assetID)
	if entry == nil || entry.ID == nil || *entry.ID != assetID {
		return nil, fmt.Errorf("equipment config %d is missing or mismatched", assetID)
	}
	switch {
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_End):
		if entry.WeaponType <= pb.CharacterWeaponType_CharacterWeaponType_Unspecified ||
			entry.WeaponType >= pb.CharacterWeaponType_CharacterWeaponType_Max || entry.AccessoryType != pb.AccessoryType_AccessoryType_Unspecified {
			return nil, fmt.Errorf("weapon config %d type is invalid", assetID)
		}
	case assetID >= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Accessory_Start) &&
		assetID <= uint32(pb.AssetID_AssetIDRange_Item_Equipment_Accessory_End):
		minimum, maximum := gameconfig.AccessoryIDRange(entry.AccessoryType)
		if minimum == 0 || assetID < minimum || assetID > maximum || entry.WeaponType != pb.CharacterWeaponType_CharacterWeaponType_Unspecified {
			return nil, fmt.Errorf("accessory config %d type %d does not match its id range", assetID, entry.AccessoryType)
		}
	case isArmorEquipmentAssetID(assetID):
		if entry.WeaponType != pb.CharacterWeaponType_CharacterWeaponType_Unspecified ||
			entry.AccessoryType != pb.AccessoryType_AccessoryType_Unspecified {
			return nil, fmt.Errorf("armor config %d type is invalid", assetID)
		}
	default:
		return nil, fmt.Errorf("equipment config %d id range is unsupported", assetID)
	}
	return entry, nil
}

func configuredWeaponEntry(assetID uint32) (*gameconfig.ItemEntry, error) {
	entry, err := configuredEquipmentEntry(assetID)
	if err != nil {
		return nil, err
	}
	if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unspecified {
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
	case pb.EquipmentType_EquipmentType_Chest:
		return &equipment.Chest
	case pb.EquipmentType_EquipmentType_Helmet:
		return &equipment.Helmet
	case pb.EquipmentType_EquipmentType_Shield:
		return &equipment.Shield
	case pb.EquipmentType_EquipmentType_Belt:
		return &equipment.Belt
	case pb.EquipmentType_EquipmentType_Boots:
		return &equipment.Boots
	case pb.EquipmentType_EquipmentType_Accessory1:
		return &equipment.Accessory1
	case pb.EquipmentType_EquipmentType_Accessory2:
		return &equipment.Accessory2
	default:
		return nil
	}
}

func equipmentBaseAttributeRange(entry *gameconfig.ItemEntry, key pb.EquipmentRecordAttribute) (int32, int32, bool) {
	if entry == nil {
		return 0, 0, false
	}
	switch key {
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Attack:
		return entry.AttackMin, entry.AttackMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Defence:
		return entry.DefenceMin, entry.DefenceMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Quick:
		return entry.QuickMin, entry.QuickMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxHP:
		return entry.HPMin, entry.HPMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxMP:
		return entry.MPMin, entry.MPMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Luck:
		return entry.LuckMin, entry.LuckMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Charm:
		return entry.CharmMin, entry.CharmMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Avoid:
		return entry.AvoidMin, entry.AvoidMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_PoisonResistance:
		return entry.PoisonMin, entry.PoisonMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ParalysisResistance:
		return entry.ParalysisMin, entry.ParalysisMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance:
		return entry.SleepMin, entry.SleepMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance:
		return entry.StoneMin, entry.StoneMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance:
		return entry.DrunkMin, entry.DrunkMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance:
		return entry.ConfusionMin, entry.ConfusionMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Critical:
		return entry.CriticalMin, entry.CriticalMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Counter:
		return entry.CounterModifierMin, entry.CounterModifierMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DamageBonusPercent:
		return entry.DamageBonusPercentMin, entry.DamageBonusPercentMax, true
	case pb.EquipmentRecordAttribute_EquipmentRecordAttribute_CritDamageBonusPercent:
		return entry.CritDamageBonusPercentMin, entry.CritDamageBonusPercentMax, true
	default:
		return 0, 0, false
	}
}

// newEquipmentRecord 在装备创建时独立随机各项配置范围, 并仅保存非0基础属性和有效元素属性.
func newEquipmentRecord(equipmentUUID uint64, assetID uint32) (*pb.EquipmentRecord, error) {
	if equipmentUUID == 0 {
		return nil, fmt.Errorf("equipment uuid is empty")
	}
	entry, err := configuredEquipmentEntry(assetID)
	if err != nil {
		return nil, err
	}
	recordBaseMap := make(map[int32]int64, len(equipmentBaseAttributeKeys))
	for _, key := range equipmentBaseAttributeKeys {
		minimum, maximum, ok := equipmentBaseAttributeRange(entry, key)
		if !ok || minimum > maximum {
			return nil, fmt.Errorf("equipment %d base attribute %s range is invalid", assetID, key)
		}
		width := uint64(int64(maximum) - int64(minimum))
		value := int64(minimum) + int64(xutil.RandomU64(0, width))
		if value != 0 {
			recordBaseMap[int32(key)] = value
		}
	}
	if entry.AttributeValue != 0 {
		if entry.Attribute < uint32(pb.AssetElemental_AssetElemental_Earth) || entry.Attribute >= uint32(pb.AssetElemental_AssetElemental_Max) || entry.AttributeValue > uint32(pb.Constants_Constants_Elemental_Total_Point) {
			return nil, fmt.Errorf("equipment %d attached element %d value %d is invalid", assetID, entry.Attribute, entry.AttributeValue)
		}
	}
	record := &pb.EquipmentRecord{
		Uuid:          equipmentUUID,
		AssetId:       assetID,
		RecordBaseMap: recordBaseMap,
	}
	if entry.AttributeValue != 0 {
		record.ElementAttribute = &pb.EquipmentElementAttribute{
			Element: pb.AssetElemental(entry.Attribute),
			Value:   entry.AttributeValue,
		}
	}
	return record, nil
}

// validateEquipmentRecord 校验装备实例的基础属性、附加修正、实例技能和元素属性.
func validateEquipmentRecord(record *pb.EquipmentRecord, expectedUUID uint64) error {
	if record == nil || expectedUUID == 0 || record.GetUuid() != expectedUUID {
		return fmt.Errorf("equipment key %d does not match record uuid %d", expectedUUID, record.GetUuid())
	}
	entry, err := configuredEquipmentEntry(record.GetAssetId())
	if err != nil {
		return err
	}
	recordBaseMap := record.GetRecordBaseMap()
	for rawKey, value := range recordBaseMap {
		if value == 0 {
			return fmt.Errorf("equipment %d record base %d explicitly stores zero", expectedUUID, rawKey)
		}
		key := pb.EquipmentRecordAttribute(rawKey)
		if _, _, ok := equipmentBaseAttributeRange(entry, key); !ok {
			return fmt.Errorf("equipment %d record base key %d is unsupported", expectedUUID, rawKey)
		}
	}
	for _, key := range equipmentBaseAttributeKeys {
		value := recordBaseMap[int32(key)]
		minimum, maximum, ok := equipmentBaseAttributeRange(entry, key)
		if !ok || value < int64(minimum) || value > int64(maximum) {
			return fmt.Errorf("equipment %d base attribute %s value %d is outside [%d,%d]", expectedUUID, key, value, minimum, maximum)
		}
	}
	for rawKey, value := range record.GetRecordModifierMap() {
		if value == 0 {
			return fmt.Errorf("equipment %d record modifier %d explicitly stores zero", expectedUUID, rawKey)
		}
		if _, _, ok := equipmentBaseAttributeRange(entry, pb.EquipmentRecordAttribute(rawKey)); !ok {
			return fmt.Errorf("equipment %d record modifier key %d is unsupported", expectedUUID, rawKey)
		}
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("equipment %d record modifier %d value %d is outside int32", expectedUUID, rawKey, value)
		}
	}
	if skillID := record.GetAdditionalSkillId(); skillID != 0 {
		if skillID == entry.GrantedSkillID {
			return fmt.Errorf("equipment %d additional skill %d is invalid or configured by asset", expectedUUID, skillID)
		}
		if gameconfig.GGameConfig.Skill == nil {
			return fmt.Errorf("equipment %d additional skill config is not loaded", expectedUUID)
		}
		if gameconfig.GGameConfig.Skill.Get(skillID) == nil {
			return fmt.Errorf("equipment %d additional skill %d is missing", expectedUUID, skillID)
		}
	}
	if element := record.GetElementAttribute(); element != nil {
		if element.GetElement() < pb.AssetElemental_AssetElemental_Earth || element.GetElement() >= pb.AssetElemental_AssetElemental_Max {
			return fmt.Errorf("equipment %d element %d is invalid", expectedUUID, element.GetElement())
		}
		if element.GetValue() == 0 || element.GetValue() > uint32(pb.Constants_Constants_Elemental_Total_Point) {
			return fmt.Errorf("equipment %d element value %d is invalid", expectedUUID, element.GetValue())
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
		{name: "gloves", record: equipment.GetGloves()},
	}
	for _, slot := range unsupported {
		if slot.record != nil {
			return fmt.Errorf("unsupported equipped slot %s is populated", slot.name)
		}
	}
	accessoryType := pb.AccessoryType_AccessoryType_Unspecified
	for _, equipmentType := range supportedCharacterEquipmentTypes {
		equipped := *characterEquipmentSlot(equipment, equipmentType)
		if equipped == nil {
			continue
		}
		if err := validateEquipmentRecord(equipped, equipped.GetUuid()); err != nil {
			return err
		}
		entry := gameconfig.GGameConfig.Item.Get(equipped.GetAssetId())
		switch equipmentType {
		case pb.EquipmentType_EquipmentType_Weapon:
			if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unspecified {
				return fmt.Errorf("weapon slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Chest:
			if !isChestEquipmentAssetID(equipped.GetAssetId()) {
				return fmt.Errorf("chest slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Helmet:
			if !isHelmetEquipmentAssetID(equipped.GetAssetId()) {
				return fmt.Errorf("helmet slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Shield:
			if !isShieldEquipmentAssetID(equipped.GetAssetId()) {
				return fmt.Errorf("shield slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Belt:
			if !isBeltEquipmentAssetID(equipped.GetAssetId()) {
				return fmt.Errorf("belt slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Boots:
			if !isBootsEquipmentAssetID(equipped.GetAssetId()) {
				return fmt.Errorf("boots slot contains incompatible equipment %d", equipped.GetAssetId())
			}
		case pb.EquipmentType_EquipmentType_Accessory1, pb.EquipmentType_EquipmentType_Accessory2:
			if entry.AccessoryType == pb.AccessoryType_AccessoryType_Unspecified {
				return fmt.Errorf("accessory slot contains incompatible equipment %d", equipped.GetAssetId())
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

// equipmentAttribute 合并装备基础属性、普通附加修正和由实例附加技能实时派生的抗性修正.
func equipmentAttribute(record *pb.EquipmentRecord, key pb.EquipmentRecordAttribute) int64 {
	if record == nil {
		return 0
	}
	return record.GetRecordBaseMap()[int32(key)] + record.GetRecordModifierMap()[int32(key)] + equipmentSkillResistanceModifier(record, key)
}

// characterEffectiveAttribute 保留当前项目裸装公式, 再按原版ITEM_equipEffect顺序叠加装备实例属性.
func characterEffectiveAttribute(record *pb.CharacterRecord) (*pb.CharacterEffectiveAttribute, error) {
	if record == nil || record.GetBase() == nil || record.GetBase().GetUuid() == 0 || record.GetEquipment() == nil {
		return nil, fmt.Errorf("character record is incomplete")
	}
	if err := validateCharacterEquipmentSlots(record.GetEquipment()); err != nil {
		return nil, err
	}
	base := record.GetBase()
	attribute := base.GetAttribute()
	elementalPoints := base.GetElemental()
	vitality := int64(attribute.GetVitality())
	strength := int64(attribute.GetStrength())
	toughness := int64(attribute.GetToughness())
	dexterity := int64(attribute.GetDexterity())
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
	critDamageBonusPercent := int64(0)
	paralysis := int64(0)
	sleep := int64(0)
	stone := int64(0)
	drunk := int64(0)
	confusion := int64(0)
	damageBonusPercent := int64(0)
	// 角色基础值和装备attribvalue统一使用原版0至100百分比单位.
	// 按ITEM_equipEffect直接叠加装备值, 无需在协议边界换算.
	elemental := [4]int64{
		int64(elementalPoints.GetEarth()),
		int64(elementalPoints.GetWater()),
		int64(elementalPoints.GetFire()),
		int64(elementalPoints.GetWind()),
	}
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
		maxHP += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxHP)
		attack += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Attack)
		defense += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Defence)
		agility += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Quick)
		maxMP += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_MaxMP)
		luck += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Luck)
		charm += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Charm)
		avoid += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Avoid)
		paralysis += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ParalysisResistance)
		sleep += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_SleepResistance)
		stone += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_StoneResistance)
		drunk += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DrunkResistance)
		confusion += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_ConfusionResistance)
		critical += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_Critical)
		critDamageBonusPercent += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_CritDamageBonusPercent)
		damageBonusPercent += equipmentAttribute(equipped, pb.EquipmentRecordAttribute_EquipmentRecordAttribute_DamageBonusPercent) + int64(entry.OtherDamage)
		if elementAttribute := equipped.GetElementAttribute(); elementAttribute != nil {
			selected := int(elementAttribute.GetElement() - 1)
			value := int64(elementAttribute.GetValue())
			for index := range elemental {
				if index == selected {
					elemental[index] += value
				} else {
					elemental[index] -= value
				}
			}
		}
	}

	for index := range elemental {
		elemental[index] = clampEquipmentValue(elemental[index], 0, int64(pb.Constants_Constants_Elemental_Total_Point))
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
		CritDamageBonusPercent:      int32(clampEquipmentValue(critDamageBonusPercent, math.MinInt32, math.MaxInt32)),
		ParalysisResistanceModifier: int32(clampEquipmentValue(paralysis, math.MinInt32, math.MaxInt32)),
		SleepResistanceModifier:     int32(clampEquipmentValue(sleep, math.MinInt32, math.MaxInt32)),
		StoneResistanceModifier:     int32(clampEquipmentValue(stone, math.MinInt32, math.MaxInt32)),
		DrunkResistanceModifier:     int32(clampEquipmentValue(drunk, math.MinInt32, math.MaxInt32)),
		ConfusionResistanceModifier: int32(clampEquipmentValue(confusion, math.MinInt32, math.MaxInt32)),
		Elemental:                   &pb.ElementalPoints{Earth: uint32(elemental[0]), Water: uint32(elemental[1]), Fire: uint32(elemental[2]), Wind: uint32(elemental[3])},
		DamageBonusPercent:          int32(clampEquipmentValue(damageBonusPercent, math.MinInt32, math.MaxInt32)),
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
	characterUUID uint64
	equipmentType pb.EquipmentType
	characterSlot int
	// equipmentUUID 为 0 表示卸下当前部位装备.
	equipmentUUID  uint64
	unequipCurrent bool
	effective      *pb.CharacterEffectiveAttribute
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
	if characterRecord.ItemBag == nil || characterRecord.Equipment == nil {
		return nil, fmt.Errorf("%w: character equipment container is missing", errCharacterEquipmentRecordInvalid)
	}

	// 换装只校验与计算, 不修改权威档案. 有效属性仅依赖 base 与 equipment,
	// 因此用一份装备槽位浅拷贝预演, 不克隆整个角色档案.
	probeEquipment := proto.Clone(characterRecord.GetEquipment()).(*pb.CharacterEquipmentRecord)
	targetSlot := characterEquipmentSlot(characterRecord.Equipment, equipmentType)
	currentEquipment := *targetSlot
	if equipmentUUID == 0 {
		if currentEquipment == nil {
			return nil, fmt.Errorf("%w: equipment slot is already empty", errCharacterEquipmentFailedPrecondition)
		}
		if itemContainerCount(characterRecord.GetItemBag()) >= int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount) {
			return nil, errCharacterEquipmentResourceExhausted
		}
		if _, exists := characterRecord.ItemBag.GetEquipmentRecordMap()[currentEquipment.GetUuid()]; exists {
			return nil, fmt.Errorf("%w: equipment %d already exists in bag", errCharacterEquipmentRecordInvalid, currentEquipment.GetUuid())
		}
		*characterEquipmentSlot(probeEquipment, equipmentType) = nil
	} else {
		nextEquipment := characterRecord.ItemBag.GetEquipmentRecordMap()[equipmentUUID]
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
		switch equipmentType {
		case pb.EquipmentType_EquipmentType_Weapon:
			if entry.WeaponType == pb.CharacterWeaponType_CharacterWeaponType_Unspecified {
				return nil, fmt.Errorf("%w: equipment cannot enter weapon slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Chest:
			if !isChestEquipmentAssetID(nextEquipment.GetAssetId()) {
				return nil, fmt.Errorf("%w: equipment cannot enter chest slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Helmet:
			if !isHelmetEquipmentAssetID(nextEquipment.GetAssetId()) {
				return nil, fmt.Errorf("%w: equipment cannot enter helmet slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Shield:
			if !isShieldEquipmentAssetID(nextEquipment.GetAssetId()) {
				return nil, fmt.Errorf("%w: equipment cannot enter shield slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Belt:
			if !isBeltEquipmentAssetID(nextEquipment.GetAssetId()) {
				return nil, fmt.Errorf("%w: equipment cannot enter belt slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Boots:
			if !isBootsEquipmentAssetID(nextEquipment.GetAssetId()) {
				return nil, fmt.Errorf("%w: equipment cannot enter boots slot", errCharacterEquipmentFailedPrecondition)
			}
		case pb.EquipmentType_EquipmentType_Accessory1, pb.EquipmentType_EquipmentType_Accessory2:
			if entry.AccessoryType == pb.AccessoryType_AccessoryType_Unspecified {
				return nil, fmt.Errorf("%w: equipment cannot enter accessory slot", errCharacterEquipmentFailedPrecondition)
			}
			otherAccessory := characterRecord.Equipment.GetAccessory1()
			if equipmentType == pb.EquipmentType_EquipmentType_Accessory1 {
				otherAccessory = characterRecord.Equipment.GetAccessory2()
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
		characterLevel, err := gameconfig.GGameConfig.Exp.GetLevel(characterRecord.GetBase().GetExp())
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
		if currentEquipment != nil {
			if _, exists := characterRecord.ItemBag.GetEquipmentRecordMap()[currentEquipment.GetUuid()]; exists {
				return nil, fmt.Errorf("%w: current equipment %d already exists in bag", errCharacterEquipmentRecordInvalid, currentEquipment.GetUuid())
			}
		}
		*characterEquipmentSlot(probeEquipment, equipmentType) = nextEquipment
	}

	effective, err := characterEffectiveAttribute(&pb.CharacterRecord{
		Base:      characterRecord.GetBase(),
		Equipment: probeEquipment,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: effective attribute: %v", errCharacterEquipmentRecordInvalid, err)
	}
	return &characterEquipmentReplacePlan{
		characterUUID:  characterRecord.GetBase().GetUuid(),
		equipmentType:  equipmentType,
		characterSlot:  characterSlot,
		equipmentUUID:  equipmentUUID,
		unequipCurrent: equipmentUUID == 0,
		effective:      effective,
	}, nil
}

// applyCharacterEquipmentReplacePlan 把换装结果原地应用到权威角色档案, 再通知落盘。
// 与改造前的区别: 不再克隆整个 AccountRecord, 也不再做事后差分解算通知。
func applyCharacterEquipmentReplacePlan(plan *characterEquipmentReplacePlan, accountRecord *pb.AccountRecord, character *character, persist func() error) error {
	if plan == nil || accountRecord == nil || character == nil || persist == nil || character.record == nil {
		return errCharacterEquipmentInvalidArgument
	}
	if plan.characterSlot < 0 || plan.characterSlot >= len(accountRecord.GetCharacterRecordList()) ||
		accountRecord.GetCharacterRecordList()[plan.characterSlot] != character.record {
		return fmt.Errorf("%w: authoritative character changed before persistence", errCharacterEquipmentRecordInvalid)
	}
	if character.record.ItemBag == nil || character.record.Equipment == nil {
		return fmt.Errorf("%w: character equipment container is missing", errCharacterEquipmentRecordInvalid)
	}
	if character.record.ItemBag.EquipmentRecordMap == nil {
		character.record.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}
	targetSlot := characterEquipmentSlot(character.record.Equipment, plan.equipmentType)
	if targetSlot == nil {
		return fmt.Errorf("%w: character equipment slot is unavailable", errCharacterEquipmentRecordInvalid)
	}
	currentEquipment := *targetSlot

	if plan.unequipCurrent {
		if currentEquipment == nil {
			return fmt.Errorf("%w: equipment slot is already empty", errCharacterEquipmentFailedPrecondition)
		}
		character.record.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()] = currentEquipment
		*targetSlot = nil
		return persist()
	}

	nextEquipment := character.record.ItemBag.GetEquipmentRecordMap()[plan.equipmentUUID]
	if nextEquipment == nil {
		return fmt.Errorf("%w: equipment %d is not in character bag", errCharacterEquipmentTargetNotFound, plan.equipmentUUID)
	}
	delete(character.record.ItemBag.EquipmentRecordMap, plan.equipmentUUID)
	if currentEquipment != nil {
		character.record.ItemBag.EquipmentRecordMap[currentEquipment.GetUuid()] = currentEquipment
	}
	*targetSlot = nextEquipment
	return persist()
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
	if err := applyCharacterEquipmentReplacePlan(plan, p.accountRecord, character, p.deferAccountRecordPersist); err != nil {
		xlog.GLog.Errorf("persist character equipment replace failed aid:%d character:%d type:%s equipment:%d err:%v", p.aid, request.GetCharacterUuid(), request.GetEquipmentType(), request.GetEquipmentUuid(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.Internal.Code())
		return
	}
	// 响应只携带本次替换部位的装备; 卸下目标部位装备时该字段保持未设置.
	var replacedEquipment *pb.EquipmentRecord
	if equipped := *characterEquipmentSlot(character.record.GetEquipment(), plan.equipmentType); equipped != nil {
		replacedEquipment = proto.Clone(equipped).(*pb.EquipmentRecord)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CharacterEquipmentReplaceRes_CMD), xerror.Success.Code(), &pb.CharacterEquipmentReplaceRes{
		CharacterUuid:      plan.characterUUID,
		EquipmentType:      plan.equipmentType,
		ItemBag:            proto.Clone(character.record.GetItemBag()).(*pb.ItemContainerRecord),
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

func equipmentAttributeValueInt32(record *pb.EquipmentRecord, key pb.EquipmentRecordAttribute) int32 {
	value := equipmentAttribute(record, key)
	if value < math.MinInt32 {
		return math.MinInt32
	}
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(value)
}
