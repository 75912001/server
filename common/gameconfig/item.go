package gameconfig

import (
	"strings"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type ItemUseTarget string

const (
	ItemUseTargetCharacter ItemUseTarget = "character"
	ItemUseTargetPet       ItemUseTarget = "pet"
)

type ItemConfig struct {
	*xmap.MapMgr[uint32, *ItemEntry]
}

type ItemEntry struct {
	ID            *uint32                `yaml:"-"`
	Name          *string                `yaml:"name"`
	SecretName    string                 `yaml:"secretname"`
	EffectString  string                 `yaml:"effectstring"`
	Atlas         *string                `yaml:"atlas"`
	Sprite        *uint32                `yaml:"sprite"`
	IDTier        uint32                 `yaml:"idTier"`
	Level         uint32                 `yaml:"level"`
	Profession    pb.CharacterProfession `yaml:"neprof"`
	OtherDamage   int32                  `yaml:"otdmags"`
	OtherDefence  int32                  `yaml:"otdefcs"`
	SetID         uint32                 `yaml:"setId"`
	WeaponType    pb.CharacterWeaponType `yaml:"-"`
	AccessoryType pb.AccessoryType       `yaml:"accessory_type"`

	AttackNumberMin uint32 `yaml:"attacknum_min"`
	AttackNumberMax uint32 `yaml:"attacknum_max"`
	AttackMin       int32  `yaml:"attack_min"`
	AttackMax       int32  `yaml:"attack_max"`
	DefenceMin      int32  `yaml:"defence_min"`
	DefenceMax      int32  `yaml:"defence_max"`
	QuickMin        int32  `yaml:"quick_min"`
	QuickMax        int32  `yaml:"quick_max"`
	HPMin           int32  `yaml:"hp_min"`
	HPMax           int32  `yaml:"hp_max"`
	MPMin           int32  `yaml:"mp_min"`
	MPMax           int32  `yaml:"mp_max"`
	LuckMin         int32  `yaml:"luck_min"`
	LuckMax         int32  `yaml:"luck_max"`
	CharmMin        int32  `yaml:"charm_min"`
	CharmMax        int32  `yaml:"charm_max"`
	AvoidMin        int32  `yaml:"avoid_min"`
	AvoidMax        int32  `yaml:"avoid_max"`

	Attribute                 uint32 `yaml:"attrib"`
	AttributeValue            uint32 `yaml:"attribvalue"`
	PoisonMin                 int32  `yaml:"poison_min"`
	PoisonMax                 int32  `yaml:"poison_max"`
	ParalysisMin              int32  `yaml:"paralysis_min"`
	ParalysisMax              int32  `yaml:"paralysis_max"`
	SleepMin                  int32  `yaml:"sleep_min"`
	SleepMax                  int32  `yaml:"sleep_max"`
	StoneMin                  int32  `yaml:"stone_min"`
	StoneMax                  int32  `yaml:"stone_max"`
	DrunkMin                  int32  `yaml:"drunk_min"`
	DrunkMax                  int32  `yaml:"drunk_max"`
	ConfusionMin              int32  `yaml:"confusion_min"`
	ConfusionMax              int32  `yaml:"confusion_max"`
	CriticalMin               int32  `yaml:"critical_min"`
	CriticalMax               int32  `yaml:"critical_max"`
	CounterModifierMin        int32  `yaml:"counter_modifier_min"`
	CounterModifierMax        int32  `yaml:"counter_modifier_max"`
	DamageBonusPercentMin     int32  `yaml:"damage_bonus_percent_min"`
	DamageBonusPercentMax     int32  `yaml:"damage_bonus_percent_max"`
	CritDamageBonusPercentMin int32  `yaml:"crit_damage_bonus_percent_min"`
	CritDamageBonusPercentMax int32  `yaml:"crit_damage_bonus_percent_max"`
	// GrantedSkillID只表示装备授予的现代技能ID, 耗蓝由技能.yaml中的技能独立配置.
	GrantedSkillID uint32 `yaml:"magicid"`

	Use *ItemUseEntry `yaml:"use"`

	legacyRangeFields bool `yaml:"-"`
}

type itemEntryYAML ItemEntry

type itemRangeYAML struct {
	AttackNumber           []uint32 `yaml:"attacknum"`
	Attack                 []int32  `yaml:"attack"`
	Defence                []int32  `yaml:"defence"`
	Quick                  []int32  `yaml:"quick"`
	HP                     []int32  `yaml:"hp"`
	MP                     []int32  `yaml:"mp"`
	Luck                   []int32  `yaml:"luck"`
	Charm                  []int32  `yaml:"charm"`
	Avoid                  []int32  `yaml:"avoid"`
	Poison                 []int32  `yaml:"poison"`
	Paralysis              []int32  `yaml:"paralysis"`
	Sleep                  []int32  `yaml:"sleep"`
	Stone                  []int32  `yaml:"stone"`
	Drunk                  []int32  `yaml:"drunk"`
	Confusion              []int32  `yaml:"confusion"`
	Critical               []int32  `yaml:"critical"`
	CounterModifier        []int32  `yaml:"counter_modifier"`
	DamageBonusPercent     []int32  `yaml:"damage_bonus_percent"`
	CritDamageBonusPercent []int32  `yaml:"crit_damage_bonus_percent"`
}

// UnmarshalYAML 读取武器的二元素范围数组, 并继续兼容七类装备尚未迁移的 Min/Max 字段.
func (p *ItemEntry) UnmarshalYAML(node *yaml.Node) error {
	var legacy itemEntryYAML
	if err := node.Decode(&legacy); err != nil {
		return err
	}
	var ranges itemRangeYAML
	if err := node.Decode(&ranges); err != nil {
		return err
	}
	*p = ItemEntry(legacy)
	p.legacyRangeFields = hasLegacyItemRangeFields(node)
	if err := assignUint32Range("attacknum", ranges.AttackNumber, &p.AttackNumberMin, &p.AttackNumberMax); err != nil {
		return err
	}
	for _, definition := range []struct {
		name   string
		values []int32
		min    *int32
		max    *int32
	}{
		{name: "attack", values: ranges.Attack, min: &p.AttackMin, max: &p.AttackMax},
		{name: "defence", values: ranges.Defence, min: &p.DefenceMin, max: &p.DefenceMax},
		{name: "quick", values: ranges.Quick, min: &p.QuickMin, max: &p.QuickMax},
		{name: "hp", values: ranges.HP, min: &p.HPMin, max: &p.HPMax},
		{name: "mp", values: ranges.MP, min: &p.MPMin, max: &p.MPMax},
		{name: "luck", values: ranges.Luck, min: &p.LuckMin, max: &p.LuckMax},
		{name: "charm", values: ranges.Charm, min: &p.CharmMin, max: &p.CharmMax},
		{name: "avoid", values: ranges.Avoid, min: &p.AvoidMin, max: &p.AvoidMax},
		{name: "poison", values: ranges.Poison, min: &p.PoisonMin, max: &p.PoisonMax},
		{name: "paralysis", values: ranges.Paralysis, min: &p.ParalysisMin, max: &p.ParalysisMax},
		{name: "sleep", values: ranges.Sleep, min: &p.SleepMin, max: &p.SleepMax},
		{name: "stone", values: ranges.Stone, min: &p.StoneMin, max: &p.StoneMax},
		{name: "drunk", values: ranges.Drunk, min: &p.DrunkMin, max: &p.DrunkMax},
		{name: "confusion", values: ranges.Confusion, min: &p.ConfusionMin, max: &p.ConfusionMax},
		{name: "critical", values: ranges.Critical, min: &p.CriticalMin, max: &p.CriticalMax},
		{name: "counter_modifier", values: ranges.CounterModifier, min: &p.CounterModifierMin, max: &p.CounterModifierMax},
		{name: "damage_bonus_percent", values: ranges.DamageBonusPercent, min: &p.DamageBonusPercentMin, max: &p.DamageBonusPercentMax},
		{name: "crit_damage_bonus_percent", values: ranges.CritDamageBonusPercent, min: &p.CritDamageBonusPercentMin, max: &p.CritDamageBonusPercentMax},
	} {
		if err := assignInt32Range(definition.name, definition.values, definition.min, definition.max); err != nil {
			return err
		}
	}
	return nil
}

func hasLegacyItemRangeFields(node *yaml.Node) bool {
	legacyFields := map[string]struct{}{
		"attacknum_min": {}, "attacknum_max": {}, "attack_min": {}, "attack_max": {},
		"defence_min": {}, "defence_max": {}, "quick_min": {}, "quick_max": {},
		"hp_min": {}, "hp_max": {}, "mp_min": {}, "mp_max": {},
		"luck_min": {}, "luck_max": {}, "charm_min": {}, "charm_max": {},
		"avoid_min": {}, "avoid_max": {}, "poison_min": {}, "poison_max": {},
		"paralysis_min": {}, "paralysis_max": {}, "sleep_min": {}, "sleep_max": {},
		"stone_min": {}, "stone_max": {}, "drunk_min": {}, "drunk_max": {},
		"confusion_min": {}, "confusion_max": {}, "critical_min": {}, "critical_max": {},
		"counter_modifier_min": {}, "counter_modifier_max": {},
		"damage_bonus_percent_min": {}, "damage_bonus_percent_max": {},
		"crit_damage_bonus_percent_min": {}, "crit_damage_bonus_percent_max": {},
	}
	if node.Kind != yaml.MappingNode {
		return false
	}
	for index := 0; index < len(node.Content); index += 2 {
		if _, ok := legacyFields[node.Content[index].Value]; ok {
			return true
		}
	}
	return false
}

func assignUint32Range(name string, values []uint32, minimum, maximum *uint32) error {
	if values == nil {
		return nil
	}
	if len(values) != 2 {
		return errors.Errorf("%s必须是包含最小值和最大值的二元素数组", name)
	}
	*minimum, *maximum = values[0], values[1]
	return nil
}

func assignInt32Range(name string, values []int32, minimum, maximum *int32) error {
	if values == nil {
		return nil
	}
	if len(values) != 2 {
		return errors.Errorf("%s必须是包含最小值和最大值的二元素数组", name)
	}
	*minimum, *maximum = values[0], values[1]
	return nil
}

type ItemUseEntry struct {
	Target  *ItemUseTarget `yaml:"target"`
	Exp     *uint64        `yaml:"exp"`
	Loyalty *uint32        `yaml:"loyalty"`
}

type itemGroupDefinition struct {
	name       string
	fileName   string
	start      uint32
	end        uint32
	weapon     bool
	equipment  bool
	allowEmpty bool
	weaponType pb.CharacterWeaponType
}

var itemGroupDefinitions = []itemGroupDefinition{
	{name: "item", fileName: FileItem, start: uint32(pb.AssetID_AssetIDRange_Item_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Material_Start) - 1},
	{name: "material", fileName: FileItemMaterial, start: uint32(pb.AssetID_AssetIDRange_Item_Material_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Material_End)},
	{name: "currency", fileName: FileItemCurrency, start: uint32(pb.AssetID_AssetIDRange_Item_Currency_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Currency_End)},
	{name: "equipmentChest", fileName: FileItemEquipmentChest, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Chest_End), equipment: true, allowEmpty: true},
	{name: "equipmentHelmet", fileName: FileItemEquipmentHelmet, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Helmet_End), equipment: true, allowEmpty: true},
	{name: "equipmentShield", fileName: FileItemEquipmentShield, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Shield_End), equipment: true, allowEmpty: true},
	{name: "equipmentGloves", fileName: FileItemEquipmentGloves, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Gloves_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Gloves_End), equipment: true, allowEmpty: true},
	{name: "equipmentBelt", fileName: FileItemEquipmentBelt, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Belt_End), equipment: true, allowEmpty: true},
	{name: "equipmentBoots", fileName: FileItemEquipmentBoots, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Boots_End), equipment: true, allowEmpty: true},
	{name: "equipmentAccessory", fileName: FileItemEquipmentAccessory, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Accessory_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Accessory_End), equipment: true, allowEmpty: true},
	{name: "weaponClaw", fileName: FileItemWeaponClaw, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Claw_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Claw_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Claw},
	{name: "weaponAxe", fileName: FileItemWeaponAxe, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Axe_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Axe_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Axe},
	{name: "weaponStaff", fileName: FileItemWeaponStaff, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Staff_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Staff_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Stick},
	{name: "weaponSpear", fileName: FileItemWeaponSpear, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Spear_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Spear_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Spear},
	{name: "weaponBow", fileName: FileItemWeaponBow, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Bow_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Bow_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Bow},
	{name: "weaponBoomerang", fileName: FileItemWeaponBoomerang, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Boomerang_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_Boomerang_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Boomerang},
	{name: "weaponThrowingAxe", fileName: FileItemWeaponThrowingAxe, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_ThrowingAxe_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_ThrowingAxe_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_ThrowingAxe},
	{name: "weaponThrowingStone", fileName: FileItemWeaponThrowingStone, start: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_ThrowingStone_Start), end: uint32(pb.AssetID_AssetIDRange_Item_Equipment_Weapon_ThrowingStone_End), weapon: true, equipment: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_ThrowingStone},
}

func newItemConfig() *ItemConfig {
	return &ItemConfig{MapMgr: xmap.NewMapMgr[uint32, *ItemEntry]()}
}

func loadItemGroups(dir string, fileName string) (map[string]map[uint32]*ItemEntry, error) {
	var root struct {
		Items map[string]map[uint32]*ItemEntry `yaml:"items"`
	}
	if err := loadYAMLFile(dir, fileName, &root); err != nil {
		return nil, err
	}
	if len(root.Items) == 0 {
		return nil, errors.Errorf("道具配置不能为空: %s %v", fileName, xruntime.Location())
	}
	for groupName, entries := range root.Items {
		group, ok := findItemGroupDefinition(groupName)
		if !ok {
			return nil, errors.Errorf("道具分组无效: file:%s group:%s %v", fileName, groupName, xruntime.Location())
		}
		if group.fileName != fileName {
			return nil, errors.Errorf("道具分组所属文件错误: file:%s group:%s %v", fileName, groupName, xruntime.Location())
		}
		if entries == nil || (len(entries) == 0 && !group.allowEmpty) {
			return nil, errors.Errorf("道具分组不能为空: file:%s group:%s %v", fileName, groupName, xruntime.Location())
		}
	}
	return root.Items, nil
}

func (p *ItemConfig) load(dir string) error {
	itemGroups := make(map[string]map[uint32]*ItemEntry)
	fileNames := append([]string{FileItem, FileItemMaterial, FileItemCurrency}, FileItemWeapons...)
	fileNames = append(fileNames, FileItemEquipments...)
	for _, fileName := range fileNames {
		groups, err := loadItemGroups(dir, fileName)
		if err != nil {
			return err
		}
		for groupName, entries := range groups {
			if _, exists := itemGroups[groupName]; exists {
				return errors.Errorf("道具分组跨文件重复: group:%s %v", groupName, xruntime.Location())
			}
			itemGroups[groupName] = entries
		}
	}

	seenItemIDs := make(map[uint32]string)
	for _, group := range itemGroupDefinitions {
		entries, ok := itemGroups[group.name]
		if !ok {
			continue
		}
		for itemID, entry := range entries {
			if itemID < group.start || itemID > group.end {
				return errors.Errorf("道具ID不属于配置分组: group:%s id:%d range:[%d,%d] %v", group.name, itemID, group.start, group.end, xruntime.Location())
			}
			if existingGroup, exists := seenItemIDs[itemID]; exists {
				return errors.Errorf("道具ID跨分组重复: id:%d group:%s duplicateGroup:%s %v", itemID, existingGroup, group.name, xruntime.Location())
			}
			seenItemIDs[itemID] = group.name
			if entry == nil {
				return errors.Errorf("道具配置不能为空: group:%s id:%d %v", group.name, itemID, xruntime.Location())
			}
			if group.weapon && entry.legacyRangeFields {
				return errors.Errorf("武器范围必须使用二元素数组, 不再接受_min/_max字段: group:%s id:%d %v", group.name, itemID, xruntime.Location())
			}
			itemIDValue := itemID
			entry.ID = &itemIDValue
			entry.WeaponType = group.weaponType
			if group.name == "equipmentAccessory" {
				minimum, maximum := AccessoryIDRange(entry.AccessoryType)
				if minimum == 0 || itemID < minimum || itemID > maximum {
					return errors.Errorf("首饰类型与ID区间不匹配: id:%d accessory_type:%d range:[%d,%d] %v", itemID, entry.AccessoryType, minimum, maximum, xruntime.Location())
				}
			} else if entry.AccessoryType != pb.AccessoryType_AccessoryType_Unspecified {
				return errors.Errorf("非首饰分组不能配置首饰类型: group:%s id:%d %v", group.name, itemID, xruntime.Location())
			}
			if !group.weapon && !group.equipment && (entry.Name == nil || strings.TrimSpace(*entry.Name) == "") {
				return errors.Errorf("道具名称不能为空: id:%d %v", itemID, xruntime.Location())
			}
			// 非装备道具必须显式提供并配套 sprite/atlas; 七类装备允许按 C/S 字段归属独立省略展示字段.
			if !group.weapon {
				if group.equipment {
					if entry.Sprite != nil && *entry.Sprite == 0 {
						return errors.Errorf("装备sprite必须大于0: group:%s id:%d %v", group.name, itemID, xruntime.Location())
					}
					if entry.Atlas != nil {
						if err := validateItemAtlas(*entry.Atlas); err != nil {
							return errors.Errorf("道具atlas无效: id:%d atlas:%q err:%v %v", itemID, *entry.Atlas, err, xruntime.Location())
						}
					}
				} else {
					if entry.Sprite == nil {
						return errors.Errorf("道具sprite不能为空: id:%d %v", itemID, xruntime.Location())
					}
					if *entry.Sprite == 0 {
						if entry.Atlas != nil {
							return errors.Errorf("sprite为0的道具不能配置atlas: id:%d %v", itemID, xruntime.Location())
						}
					} else {
						if entry.Atlas == nil {
							return errors.Errorf("sprite大于0的道具必须配置atlas: id:%d %v", itemID, xruntime.Location())
						}
						if err := validateItemAtlas(*entry.Atlas); err != nil {
							return errors.Errorf("道具atlas无效: id:%d atlas:%q err:%v %v", itemID, *entry.Atlas, err, xruntime.Location())
						}
					}
				}
			}
			if err := validateItemUse(itemID, entry, group.equipment); err != nil {
				return err
			}
			if err := validateItemAttributes(itemID, entry); err != nil {
				return err
			}
			p.Add(itemID, entry)
		}
	}
	if len(seenItemIDs) == 0 {
		return errors.Errorf("道具配置没有可用条目: 普通道具、素材、货币、8个武器文件、7个装备文件 %v", xruntime.Location())
	}
	return nil
}

func validateItemAttributes(itemID uint32, entry *ItemEntry) error {
	if entry.AttackNumberMin > entry.AttackNumberMax {
		return errors.Errorf("道具攻击次数范围无效: id:%d min:%d max:%d %v", itemID, entry.AttackNumberMin, entry.AttackNumberMax, xruntime.Location())
	}
	ranges := []struct {
		name string
		min  int32
		max  int32
	}{
		{name: "attack", min: entry.AttackMin, max: entry.AttackMax},
		{name: "defence", min: entry.DefenceMin, max: entry.DefenceMax},
		{name: "quick", min: entry.QuickMin, max: entry.QuickMax},
		{name: "hp", min: entry.HPMin, max: entry.HPMax},
		{name: "mp", min: entry.MPMin, max: entry.MPMax},
		{name: "luck", min: entry.LuckMin, max: entry.LuckMax},
		{name: "charm", min: entry.CharmMin, max: entry.CharmMax},
		{name: "avoid", min: entry.AvoidMin, max: entry.AvoidMax},
		{name: "poison", min: entry.PoisonMin, max: entry.PoisonMax},
		{name: "paralysis", min: entry.ParalysisMin, max: entry.ParalysisMax},
		{name: "sleep", min: entry.SleepMin, max: entry.SleepMax},
		{name: "stone", min: entry.StoneMin, max: entry.StoneMax},
		{name: "drunk", min: entry.DrunkMin, max: entry.DrunkMax},
		{name: "confusion", min: entry.ConfusionMin, max: entry.ConfusionMax},
		{name: "critical", min: entry.CriticalMin, max: entry.CriticalMax},
		{name: "counter_modifier", min: entry.CounterModifierMin, max: entry.CounterModifierMax},
		{name: "damage_bonus_percent", min: entry.DamageBonusPercentMin, max: entry.DamageBonusPercentMax},
		{name: "crit_damage_bonus_percent", min: entry.CritDamageBonusPercentMin, max: entry.CritDamageBonusPercentMax},
	}
	for _, itemRange := range ranges {
		if itemRange.min > itemRange.max {
			return errors.Errorf("道具属性范围无效: id:%d field:%s min:%d max:%d %v", itemID, itemRange.name, itemRange.min, itemRange.max, xruntime.Location())
		}
	}
	if _, ok := pb.CharacterProfession_name[int32(entry.Profession)]; !ok {
		return errors.Errorf("道具职业限制无效: id:%d neprof:%d %v", itemID, entry.Profession, xruntime.Location())
	}
	if entry.Attribute > 4 {
		return errors.Errorf("道具元素类型无效: id:%d attrib:%d %v", itemID, entry.Attribute, xruntime.Location())
	}
	if entry.AttributeValue > uint32(pb.Constants_Constants_Elemental_Total_Point) {
		return errors.Errorf("道具元素值无效: id:%d attribvalue:%d %v", itemID, entry.AttributeValue, xruntime.Location())
	}
	if entry.Attribute == 0 && entry.AttributeValue != 0 {
		return errors.Errorf("无元素道具不能配置元素值: id:%d attribvalue:%d %v", itemID, entry.AttributeValue, xruntime.Location())
	}
	return nil
}

func findItemGroupDefinition(name string) (itemGroupDefinition, bool) {
	for _, group := range itemGroupDefinitions {
		if group.name == name {
			return group, true
		}
	}
	return itemGroupDefinition{}, false
}

func validateItemAtlas(atlas string) error {
	if atlas == "" || strings.TrimSpace(atlas) != atlas {
		return errors.New("路径为空或包含首尾空白")
	}
	if !strings.HasPrefix(atlas, "item/") {
		return errors.New("路径必须以item/开头")
	}
	if strings.Contains(atlas, "\\") || strings.Contains(atlas, ":") || strings.HasSuffix(atlas, "/") {
		return errors.New("路径格式非法")
	}
	if strings.HasSuffix(strings.ToLower(atlas), ".png") || strings.HasSuffix(strings.ToLower(atlas), ".tpsheet") {
		return errors.New("路径不能包含文件扩展名")
	}
	for _, segment := range strings.Split(atlas, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("路径包含非法段")
		}
	}
	return nil
}

func validateItemUse(itemID uint32, entry *ItemEntry, equipment bool) error {
	if equipment {
		if entry.Use != nil {
			return errors.Errorf("装备不能配置使用效果: id:%d %v", itemID, xruntime.Location())
		}
		return nil
	}
	if entry.Use == nil {
		return nil
	}
	if entry.Use.Target == nil {
		return errors.Errorf("道具使用配置不完整: id:%d %v", itemID, xruntime.Location())
	}
	switch *entry.Use.Target {
	case ItemUseTargetCharacter, ItemUseTargetPet:
	default:
		return errors.Errorf("道具使用目标无效: id:%d target:%q %v", itemID, *entry.Use.Target, xruntime.Location())
	}
	effectCount := 0
	if entry.Use.Exp != nil {
		if *entry.Use.Exp == 0 {
			return errors.Errorf("道具使用经验值必须大于0: id:%d %v", itemID, xruntime.Location())
		}
		effectCount++
	}
	if entry.Use.Loyalty != nil {
		if *entry.Use.Loyalty == 0 {
			return errors.Errorf("道具使用忠诚度必须大于0: id:%d %v", itemID, xruntime.Location())
		}
		if *entry.Use.Target != ItemUseTargetPet {
			return errors.Errorf("忠诚度道具只能用于宠物: id:%d target:%q %v", itemID, *entry.Use.Target, xruntime.Location())
		}
		effectCount++
	}
	if effectCount != 1 {
		return errors.Errorf("道具必须且只能配置一种使用效果: id:%d %v", itemID, xruntime.Location())
	}
	return nil
}

func (p *ItemConfig) check() error {
	var checkErr error
	p.Foreach(func(itemID uint32, entry *ItemEntry) bool {
		if entry == nil || entry.GrantedSkillID == 0 {
			return true
		}
		if GGameConfig == nil || GGameConfig.Skill == nil {
			checkErr = errors.Errorf("装备技能配置尚未加载: item:%d skill:%d %v", itemID, entry.GrantedSkillID, xruntime.Location())
			return false
		}
		skill := GGameConfig.Skill.Get(entry.GrantedSkillID)
		if skill == nil {
			checkErr = errors.Errorf("装备引用不存在的现代技能: item:%d skill:%d %v", itemID, entry.GrantedSkillID, xruntime.Location())
			return false
		}
		if !skill.CanBeUsedBy("character") {
			checkErr = errors.Errorf("装备技能不允许角色使用: item:%d skill:%d %v", itemID, entry.GrantedSkillID, xruntime.Location())
			return false
		}
		return true
	})
	return checkErr
}

func (p *ItemConfig) assemble() error {
	return nil
}
