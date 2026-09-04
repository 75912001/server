package gameconfig

import (
	"strings"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
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
	Cost          uint64                 `yaml:"cost"`
	Level         uint32                 `yaml:"level"`
	Profession    pb.CharacterProfession `yaml:"neprof"`
	OtherDamage   int32                  `yaml:"otdmags"`
	OtherDefence  int32                  `yaml:"otdefcs"`
	SuitCode      uint32                 `yaml:"nsuit"`
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

	Attribute      uint32 `yaml:"attrib"`
	AttributeValue uint32 `yaml:"attribvalue"`
	PoisonMin      int32  `yaml:"poison_min"`
	PoisonMax      int32  `yaml:"poison_max"`
	ParalysisMin   int32  `yaml:"paralysis_min"`
	ParalysisMax   int32  `yaml:"paralysis_max"`
	SleepMin       int32  `yaml:"sleep_min"`
	SleepMax       int32  `yaml:"sleep_max"`
	StoneMin       int32  `yaml:"stone_min"`
	StoneMax       int32  `yaml:"stone_max"`
	DrunkMin       int32  `yaml:"drunk_min"`
	DrunkMax       int32  `yaml:"drunk_max"`
	ConfusionMin   int32  `yaml:"confusion_min"`
	ConfusionMax   int32  `yaml:"confusion_max"`
	CriticalMin    int32  `yaml:"critical_min"`
	CriticalMax    int32  `yaml:"critical_max"`
	MagicID        uint32 `yaml:"magicid"`
	MagicUseMP     uint32 `yaml:"magicusemp"`

	Use *ItemUseEntry `yaml:"use"`
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
	weaponType pb.CharacterWeaponType
}

var itemGroupDefinitions = []itemGroupDefinition{
	{name: "item", fileName: FileItem, start: uint32(pb.AssetIDRange_AssetIDRange_Item_Item_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Item_End)},
	{name: "accessory", fileName: FileItemAccessory, start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Accessory_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Accessory_End)},
	{name: "weaponClaw", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Claw_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Claw_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Claw},
	{name: "weaponAxe", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Axe_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Axe_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Axe},
	{name: "weaponStaff", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Staff_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Staff_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Stick},
	{name: "weaponSpear", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Spear_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Spear_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Spear},
	{name: "weaponBow", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Bow_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Bow_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Bow},
	{name: "weaponBoomerang", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Boomerang_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_Boomerang_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_Boomerang},
	{name: "weaponThrowingAxe", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_ThrowingAxe_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_ThrowingAxe_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_ThrowingAxe},
	{name: "weaponThrowingStone", start: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_ThrowingStone_Start), end: uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Weapon_ThrowingStone_End), weapon: true, weaponType: pb.CharacterWeaponType_CharacterWeaponType_ThrowingStone},
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
		expectedFile := group.fileName
		if group.weapon {
			expectedFile = FileItemWeapon
		}
		if expectedFile != fileName {
			return nil, errors.Errorf("道具分组所属文件错误: file:%s group:%s %v", fileName, groupName, xruntime.Location())
		}
		if entries == nil || (len(entries) == 0 && group.name != "accessory") {
			return nil, errors.Errorf("道具分组不能为空: file:%s group:%s %v", fileName, groupName, xruntime.Location())
		}
	}
	return root.Items, nil
}

func (p *ItemConfig) load(dir string) error {
	itemGroups := make(map[string]map[uint32]*ItemEntry)
	for _, fileName := range []string{FileItem, FileItemWeapon, FileItemAccessory} {
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
		groupAtlas := ""
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
			itemIDValue := itemID
			entry.ID = &itemIDValue
			entry.WeaponType = group.weaponType
			if group.name == "accessory" {
				minimum, maximum := AccessoryIDRange(entry.AccessoryType)
				if minimum == 0 || itemID < minimum || itemID > maximum {
					return errors.Errorf("首饰类型与ID区间不匹配: id:%d accessory_type:%d range:[%d,%d] %v", itemID, entry.AccessoryType, minimum, maximum, xruntime.Location())
				}
			} else if entry.AccessoryType != pb.AccessoryType_AccessoryType_Unknow {
				return errors.Errorf("非首饰分组不能配置首饰类型: group:%s id:%d %v", group.name, itemID, xruntime.Location())
			}
			if entry.Name == nil || strings.TrimSpace(*entry.Name) == "" {
				return errors.Errorf("道具名称不能为空: id:%d %v", itemID, xruntime.Location())
			}
			if entry.Sprite == nil {
				return errors.Errorf("道具sprite不能为空: id:%d %v", itemID, xruntime.Location())
			}
			if *entry.Sprite == 0 {
				if group.weapon || group.name == "accessory" {
					return errors.Errorf("装备sprite必须大于0: group:%s id:%d %v", group.name, itemID, xruntime.Location())
				}
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
				if group.weapon {
					if groupAtlas == "" {
						groupAtlas = *entry.Atlas
					} else if groupAtlas != *entry.Atlas {
						return errors.Errorf("武器分组不能混用atlas: group:%s atlas:%q duplicateAtlas:%q %v", group.name, groupAtlas, *entry.Atlas, xruntime.Location())
					}
				}
			}
			if err := validateItemUse(itemID, entry, group.weapon || group.name == "accessory"); err != nil {
				return err
			}
			if err := validateItemAttributes(itemID, entry); err != nil {
				return err
			}
			p.Add(itemID, entry)
		}
	}
	if len(seenItemIDs) == 0 {
		return errors.Errorf("道具配置没有可用条目: %s,%s %v", FileItem, FileItemWeapon, xruntime.Location())
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
	if entry.AttributeValue > 10 {
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
	return nil
}

func (p *ItemConfig) assemble() error {
	return nil
}
