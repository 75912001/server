package gameconfig

import pb "server/proto/pb"

var petAttributeKeySet = stringSet(petAttributeKeys...)
var petGrowthKeys = stringSet("initNum", "lvupPointSource", "baseVital", "baseStr", "baseTough", "baseDex")

func newPetConfig() *PetConfig {
	return &PetConfig{
		byID: map[int]*PetEntry{},
	}
}

func (p *PetConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FilePet)
	if err != nil {
		return err
	}
	petNode, err := requireKey(root, "pet", FilePet)
	if err != nil {
		return err
	}
	pets, err := requireSeq(petNode, FilePet+".pet")
	if err != nil {
		return err
	}
	for i, petNode := range pets {
		path := configErrorPath(FilePet, "pet", i)
		petData, err := requireMap(petNode, path)
		if err != nil {
			return err
		}
		entry, err := p.parseEntry(petData, path)
		if err != nil {
			return err
		}
		if _, ok := p.byID[entry.ID]; ok {
			return configError("宠物ID重复: %d", entry.ID)
		}
		p.byID[entry.ID] = entry
		p.ids = append(p.ids, entry.ID)
	}
	if len(p.byID) == 0 {
		return configError("宠物配置中没有解析到 pet 数据: %s", FilePet)
	}
	return nil
}

func (p *PetConfig) parseEntry(data yamlMap, path string) (*PetEntry, error) {
	id, err := p.requireInt(data, "id", path)
	if err != nil {
		return nil, err
	}
	if !isPetID(id) {
		return nil, configError("宠物ID超出范围: %d", id)
	}
	rarity, err := p.requireInt(data, "rarity", path)
	if err != nil {
		return nil, err
	}
	if rarity < int(pb.PetRarity_PetRarity_Common) || rarity > int(pb.PetRarity_PetRarity_Mythic) {
		return nil, configError("宠物稀有度非法: ID:%d rarity:%d", id, rarity)
	}
	elemental, err := parsePetElemental(data, id, path)
	if err != nil {
		return nil, err
	}
	attribute, err := parsePetAttribute(data, id, path)
	if err != nil {
		return nil, err
	}
	growth, err := parsePetGrowth(data, id, path)
	if err != nil {
		return nil, err
	}
	skillSlots, err := parsePetSkillSlots(data, id, path)
	if err != nil {
		return nil, err
	}
	return &PetEntry{
		ID:         id,
		Rarity:     rarity,
		Elemental:  elemental,
		Attribute:  attribute,
		Growth:     growth,
		SkillSlots: skillSlots,
	}, nil
}

func (p *PetConfig) requireInt(data yamlMap, key string, path string) (int, error) {
	node, err := requireKey(data, key, path)
	if err != nil {
		return 0, err
	}
	return intScalar(node, path+"."+key)
}

func parsePetElemental(data yamlMap, petID int, path string) (map[pb.AssetElemental]int, error) {
	node, err := requireKey(data, "elemental", path)
	if err != nil {
		return nil, err
	}
	elementalData, err := requireMap(node, path+".elemental")
	if err != nil {
		return nil, err
	}
	out := make(map[pb.AssetElemental]int, len(elementOrder))
	for _, elemental := range elementOrder {
		out[elemental] = 0
	}
	for key, valueNode := range elementalData {
		elemental, ok := elementByKey[key]
		if !ok {
			return nil, configError("宠物 elemental 元素未知: ID:%d key:%s", petID, key)
		}
		value, err := intScalar(valueNode, path+".elemental."+key)
		if err != nil {
			return nil, err
		}
		out[elemental] = value
	}
	if err := checkPetElemental(petID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func checkPetElemental(petID int, elemental map[pb.AssetElemental]int) error {
	sum := 0
	activeIndexes := []int{}
	for index, elementalType := range elementOrder {
		value := elemental[elementalType]
		if value < 0 || value > 10 {
			return configError("宠物 elemental 值必须在[0,10]: ID:%d value:%d", petID, value)
		}
		sum += value
		if value > 0 {
			activeIndexes = append(activeIndexes, index)
		}
	}
	if sum != 10 {
		return configError("宠物元素分配总和须为10: ID:%d sum:%d", petID, sum)
	}
	if len(activeIndexes) != 1 && len(activeIndexes) != 2 {
		return configError("宠物 elemental 只能是单元素或两个相邻元素: ID:%d", petID)
	}
	if len(activeIndexes) == 2 {
		distance := activeIndexes[0] - activeIndexes[1]
		if distance < 0 {
			distance = -distance
		}
		wrapDistance := len(elementOrder) - 1
		if distance != 1 && distance != wrapDistance {
			return configError("宠物 elemental 两个元素必须相邻: ID:%d", petID)
		}
	}
	return nil
}

func parsePetAttribute(data yamlMap, petID int, path string) (PetAttributeEntry, error) {
	node, err := requireKey(data, "attribute", path)
	if err != nil {
		return PetAttributeEntry{}, err
	}
	attributeData, err := requireMap(node, path+".attribute")
	if err != nil {
		return PetAttributeEntry{}, err
	}
	if err := assertKnownKeys(attributeData, petAttributeKeySet, path+".attribute"); err != nil {
		return PetAttributeEntry{}, err
	}
	values := map[string]int{}
	for _, key := range petAttributeKeys {
		valueNode, err := requireKey(attributeData, key, path+".attribute")
		if err != nil {
			return PetAttributeEntry{}, err
		}
		values[key], err = intScalar(valueNode, path+".attribute."+key)
		if err != nil {
			return PetAttributeEntry{}, configError("宠物 attribute 字段非法: ID:%d key:%s err:%v", petID, key, err)
		}
	}
	return PetAttributeEntry{
		PoisonResist:    values["poisonResist"],
		ParalysisResist: values["paralysisResist"],
		SleepResist:     values["sleepResist"],
		StoneResist:     values["stoneResist"],
		DrunkResist:     values["drunkResist"],
		ConfusionResist: values["confusionResist"],
		Critical:        values["critical"],
		Counter:         values["counter"],
	}, nil
}

func parsePetGrowth(data yamlMap, petID int, path string) (PetGrowthEntry, error) {
	node, err := requireKey(data, "growth", path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	growthData, err := requireMap(node, path+".growth")
	if err != nil {
		return PetGrowthEntry{}, err
	}
	if err := assertKnownKeys(growthData, petGrowthKeys, path+".growth"); err != nil {
		return PetGrowthEntry{}, err
	}
	initNum, err := requireNonNegativeGrowthInt(growthData, "initNum", petID, path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	lvupNode, err := requireKey(growthData, "lvupPointSource", path+".growth")
	if err != nil {
		return PetGrowthEntry{}, err
	}
	lvupPointSource, err := floatScalar(lvupNode, path+".growth.lvupPointSource")
	if err != nil {
		return PetGrowthEntry{}, err
	}
	if lvupPointSource <= 0 {
		return PetGrowthEntry{}, configError("宠物 growth.lvupPointSource 必须大于0: ID:%d value:%v", petID, lvupPointSource)
	}
	baseVital, err := requireNonNegativeGrowthInt(growthData, "baseVital", petID, path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	baseStr, err := requireNonNegativeGrowthInt(growthData, "baseStr", petID, path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	baseTough, err := requireNonNegativeGrowthInt(growthData, "baseTough", petID, path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	baseDex, err := requireNonNegativeGrowthInt(growthData, "baseDex", petID, path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	for key, value := range map[string]int{
		"baseVital": baseVital,
		"baseStr":   baseStr,
		"baseTough": baseTough,
		"baseDex":   baseDex,
	} {
		if value+petSavedBaseRandomMin <= 0 {
			return PetGrowthEntry{}, configError("宠物 growth.%s 加随机最小偏移后必须大于0: ID:%d value:%d min:%d", key, petID, value, petSavedBaseRandomMin)
		}
	}
	return PetGrowthEntry{
		InitNum:         initNum,
		LvupPointSource: lvupPointSource,
		BaseVital:       baseVital,
		BaseStr:         baseStr,
		BaseTough:       baseTough,
		BaseDex:         baseDex,
	}, nil
}

func requireNonNegativeGrowthInt(data yamlMap, key string, petID int, path string) (int, error) {
	node, err := requireKey(data, key, path+".growth")
	if err != nil {
		return 0, err
	}
	value, err := nonNegativeIntScalar(node, path+".growth."+key)
	if err != nil {
		return 0, configError("宠物 growth.%s 非法: ID:%d err:%v", key, petID, err)
	}
	return value, nil
}

func parsePetSkillSlots(data yamlMap, petID int, path string) ([]int, error) {
	node, err := requireKey(data, "skill", path)
	if err != nil {
		return nil, err
	}
	skillSlots, err := requireSeq(node, path+".skill")
	if err != nil {
		return nil, err
	}
	if len(skillSlots) == 0 {
		return nil, configError("宠物 skill 须大于 0 个槽位: ID:%d", petID)
	}
	out := make([]int, 0, len(skillSlots))
	for index, slotNode := range skillSlots {
		skillID, err := intScalar(slotNode, path+".skill["+itoa(index)+"]")
		if err != nil {
			return nil, configError("宠物 skill 槽位非法: ID:%d err:%v", petID, err)
		}
		if skillID != 0 && !isPetSkillID(skillID) {
			return nil, configError("宠物 skill 槽位ID超出范围: ID:%d skill:%d", petID, skillID)
		}
		out = append(out, skillID)
	}
	return out, nil
}

func (p *PetConfig) check(petSkill *PetSkillConfig) error {
	if petSkill == nil {
		return configError("宠物技能配置管理器不能为空")
	}
	for _, petID := range p.ids {
		pet := p.byID[petID]
		for _, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			if !petSkill.HasID(skillID) {
				return configError("宠物引用了未定义技能: pet:%d skill:%d", pet.ID, skillID)
			}
		}
	}
	return nil
}

func (p *PetConfig) assemble() error {
	return nil
}
