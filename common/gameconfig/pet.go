package gameconfig

import pb "server/proto/pb"

func newPetConfig() *PetConfig {
	return &PetConfig{
		byID: map[uint32]*PetEntry{},
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
		petID := uint32(entry.ID)
		if int(petID) != entry.ID {
			return configError("宠物ID超出范围: %d", entry.ID)
		}
		if _, ok := p.byID[petID]; ok {
			return configError("宠物ID重复: %d", entry.ID)
		}
		p.byID[petID] = entry
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
	petID := uint32(id)
	if id < 0 || int(petID) != id || !isPetID(petID) {
		return nil, configError("宠物ID超出范围: %d", id)
	}
	rarity, err := p.requireInt(data, "rarity", path)
	if err != nil {
		return nil, err
	}
	if rarity < int(pb.PetRarity_PetRarity_Common) || rarity > int(pb.PetRarity_PetRarity_Mythic) {
		return nil, configError("宠物稀有度非法: ID:%d rarity:%d", id, rarity)
	}
	elemental, err := parsePetElemental(data, petID, path)
	if err != nil {
		return nil, err
	}
	attribute, err := parsePetAttribute(data, petID, path)
	if err != nil {
		return nil, err
	}
	growth, err := parsePetGrowth(data, petID, path)
	if err != nil {
		return nil, err
	}
	skillSlots, err := parsePetSkillSlots(data, petID, path)
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

func parsePetElemental(data yamlMap, petID uint32, path string) (map[pb.AssetElemental]uint32, error) {
	node, err := requireKey(data, "elemental", path)
	if err != nil {
		return nil, err
	}
	elementalData, err := requireMap(node, path+".elemental")
	if err != nil {
		return nil, err
	}
	out := make(map[pb.AssetElemental]uint32, int(pb.AssetElemental_AssetElemental_Max-pb.AssetElemental_AssetElemental_Unknow-1))
	for elemental := pb.AssetElemental_AssetElemental_Unknow + 1; elemental < pb.AssetElemental_AssetElemental_Max; elemental++ {
		out[elemental] = 0
	}
	for key, valueNode := range elementalData {
		var elemental pb.AssetElemental
		switch key {
		case "earth":
			elemental = pb.AssetElemental_AssetElemental_Earth
		case "water":
			elemental = pb.AssetElemental_AssetElemental_Water
		case "fire":
			elemental = pb.AssetElemental_AssetElemental_Fire
		case "wind":
			elemental = pb.AssetElemental_AssetElemental_Wind
		}
		if elemental <= pb.AssetElemental_AssetElemental_Unknow || elemental >= pb.AssetElemental_AssetElemental_Max {
			return nil, configError("宠物 elemental 元素未知: ID:%d key:%s", petID, key)
		}
		value, err := intScalar(valueNode, path+".elemental."+key)
		if err != nil {
			return nil, err
		}
		if value < 0 {
			return nil, configError("宠物 elemental 值必须在[0,10]: ID:%d value:%d", petID, value)
		}
		out[elemental] = uint32(value)
	}
	if err := checkPetElemental(petID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func checkPetElemental(petID uint32, elemental map[pb.AssetElemental]uint32) error {
	sum := uint32(0)
	activeIndexes := []int{}
	for elementalType := pb.AssetElemental_AssetElemental_Unknow + 1; elementalType < pb.AssetElemental_AssetElemental_Max; elementalType++ {
		value := elemental[elementalType]
		if value > 10 {
			return configError("宠物 elemental 值必须在[0,10]: ID:%d value:%d", petID, value)
		}
		sum += value
		if value > 0 {
			index := int(elementalType - pb.AssetElemental_AssetElemental_Unknow - 1)
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
		wrapDistance := int(pb.AssetElemental_AssetElemental_Max - pb.AssetElemental_AssetElemental_Unknow - 2)
		if distance != 1 && distance != wrapDistance {
			return configError("宠物 elemental 两个元素必须相邻: ID:%d", petID)
		}
	}
	return nil
}

func parsePetAttribute(data yamlMap, petID uint32, path string) (PetAttributeEntry, error) {
	node, err := requireKey(data, "attribute", path)
	if err != nil {
		return PetAttributeEntry{}, err
	}
	attributeData, err := requireMap(node, path+".attribute")
	if err != nil {
		return PetAttributeEntry{}, err
	}
	out := PetAttributeEntry{}
	for _, field := range []struct {
		key      string
		required bool
		value    *int
	}{
		{key: "poisonResist", required: true, value: &out.PoisonResist},
		{key: "paralysisResist", required: true, value: &out.ParalysisResist},
		{key: "sleepResist", required: true, value: &out.SleepResist},
		{key: "stoneResist", required: true, value: &out.StoneResist},
		{key: "drunkResist", required: true, value: &out.DrunkResist},
		{key: "confusionResist", required: true, value: &out.ConfusionResist},
		{key: "critical", required: true, value: &out.Critical},
		{key: "counter", required: true, value: &out.Counter},
		{key: "get", value: &out.Get},
		{key: "rate", value: &out.Rare},
	} {
		valueNode, ok := attributeData[field.key]
		if !ok {
			if !field.required {
				continue
			}
			return PetAttributeEntry{}, configError("配置缺少必填字段: %s.attribute.%s", path, field.key)
		}
		value, err := intScalar(valueNode, path+".attribute."+field.key)
		if err != nil {
			return PetAttributeEntry{}, configError("宠物 attribute 字段非法: ID:%d key:%s err:%v", petID, field.key, err)
		}
		*field.value = value
	}
	return out, nil
}

func parsePetGrowth(data yamlMap, petID uint32, path string) (PetGrowthEntry, error) {
	node, err := requireKey(data, "growth", path)
	if err != nil {
		return PetGrowthEntry{}, err
	}
	growthData, err := requireMap(node, path+".growth")
	if err != nil {
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
	for key, value := range map[string]uint32{
		"baseVital": baseVital,
		"baseStr":   baseStr,
		"baseTough": baseTough,
		"baseDex":   baseDex,
	} {
		if int32(value)+petSavedBaseGradeOffsetMin <= 0 {
			return PetGrowthEntry{}, configError("宠物 growth.%s 加品阶最小偏移后必须大于0: ID:%d value:%d min:%d", key, petID, value, petSavedBaseGradeOffsetMin)
		}
	}
	return PetGrowthEntry{
		InitNum:         int(initNum),
		LvupPointSource: lvupPointSource,
		BaseVital:       int(baseVital),
		BaseStr:         int(baseStr),
		BaseTough:       int(baseTough),
		BaseDex:         int(baseDex),
	}, nil
}

func requireNonNegativeGrowthInt(data yamlMap, key string, petID uint32, path string) (uint32, error) {
	node, err := requireKey(data, key, path+".growth")
	if err != nil {
		return 0, err
	}
	value, err := nonNegativeIntScalar(node, path+".growth."+key)
	if err != nil {
		return 0, configError("宠物 growth.%s 非法: ID:%d err:%v", key, petID, err)
	}
	out := uint32(value)
	if int(out) != value {
		return 0, configError("宠物 growth.%s 超出范围: ID:%d value:%d", key, petID, value)
	}
	return out, nil
}

func parsePetSkillSlots(data yamlMap, petID uint32, path string) ([]uint32, error) {
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
	out := make([]uint32, 0, len(skillSlots))
	for index, slotNode := range skillSlots {
		skillID, err := intScalar(slotNode, path+".skill["+itoa(index)+"]")
		if err != nil {
			return nil, configError("宠物 skill 槽位非法: ID:%d err:%v", petID, err)
		}
		petSkillID := uint32(skillID)
		if skillID != 0 && (skillID < 0 || int(petSkillID) != skillID || !isPetSkillID(petSkillID)) {
			return nil, configError("宠物 skill 槽位ID超出范围: ID:%d skill:%d", petID, skillID)
		}
		out = append(out, petSkillID)
	}
	return out, nil
}

func (p *PetConfig) check(petSkill *PetSkillConfig) error {
	if petSkill == nil {
		return configError("宠物技能配置管理器不能为空")
	}
	for _, petID := range p.ids {
		pet := p.byID[uint32(petID)]
		for _, skillID := range pet.SkillSlots {
			if skillID == 0 {
				continue
			}
			skillIDValue := int(skillID)
			if uint32(skillIDValue) != skillID || !petSkill.HasID(skillIDValue) {
				return configError("宠物引用了未定义技能: pet:%d skill:%d", pet.ID, skillID)
			}
		}
	}
	return nil
}

func (p *PetConfig) assemble() error {
	return nil
}
