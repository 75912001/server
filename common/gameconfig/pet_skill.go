package gameconfig

func newPetSkillConfig() *PetSkillConfig {
	return &PetSkillConfig{
		byID: map[int]*PetSkillEntry{},
	}
}

func (p *PetSkillConfig) load(dir string) error {
	root, err := loadYAMLMap(dir, FilePetSkill)
	if err != nil {
		return err
	}
	skillNode, err := requireKey(root, "skill", FilePetSkill)
	if err != nil {
		return err
	}
	skills, err := requireSeq(skillNode, FilePetSkill+".skill")
	if err != nil {
		return err
	}
	if len(skills) == 0 {
		return configError("宠物技能配置中没有解析到 skill 数据: %s", FilePetSkill)
	}
	for i, skillNode := range skills {
		path := configErrorPath(FilePetSkill, "skill", i)
		skillData, err := requireMap(skillNode, path)
		if err != nil {
			return err
		}
		idNode, err := requireKey(skillData, "id", path)
		if err != nil {
			return err
		}
		id, err := intScalar(idNode, path+".id")
		if err != nil {
			return err
		}
		if !isPetSkillID(id) {
			return configError("宠物技能ID超出范围: %d", id)
		}
		if _, ok := p.byID[id]; ok {
			return configError("宠物技能ID重复: %d", id)
		}
		nameNode, err := requireKey(skillData, "name", path)
		if err != nil {
			return err
		}
		name, err := stringScalar(nameNode, path+".name")
		if err != nil {
			return err
		}
		descriptionNode, err := requireKey(skillData, "description", path)
		if err != nil {
			return err
		}
		description, err := stringScalar(descriptionNode, path+".description")
		if err != nil {
			return err
		}

		p.byID[id] = &PetSkillEntry{ID: id, Name: name, Description: description}
		p.ids = append(p.ids, id)
	}
	return nil
}

func (p *PetSkillConfig) check() error {
	return nil
}

func (p *PetSkillConfig) assemble() error {
	return nil
}

func configErrorPath(filename string, section string, index int) string {
	return filename + "." + section + "[" + itoa(index) + "]"
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
