package gameconfig

func Load(dir string) (*Manager, error) {
	manager := &Manager{
		PetSkill:  newPetSkillConfig(),
		Pet:       newPetConfig(),
		Character: newCharacterConfig(),
		Enemy:     newEnemyGroupConfig(),
		Scene:     newSceneConfig(),
		Exp:       newExpConfig(),
	}

	if err := manager.PetSkill.load(dir); err != nil {
		return nil, err
	}
	if err := manager.Pet.load(dir); err != nil {
		return nil, err
	}
	if err := manager.Character.load(dir); err != nil {
		return nil, err
	}
	if err := manager.Enemy.load(dir); err != nil {
		return nil, err
	}
	if err := manager.Scene.load(dir); err != nil {
		return nil, err
	}
	if err := manager.Exp.load(dir); err != nil {
		return nil, err
	}

	if err := manager.PetSkill.check(); err != nil {
		return nil, err
	}
	if err := manager.Pet.check(manager.PetSkill); err != nil {
		return nil, err
	}
	if err := manager.Character.check(); err != nil {
		return nil, err
	}
	if err := manager.Enemy.check(manager.Pet); err != nil {
		return nil, err
	}
	if err := manager.Scene.check(manager.Enemy); err != nil {
		return nil, err
	}
	if err := manager.Exp.check(); err != nil {
		return nil, err
	}

	if err := manager.PetSkill.assemble(); err != nil {
		return nil, err
	}
	if err := manager.Pet.assemble(); err != nil {
		return nil, err
	}
	if err := manager.Character.assemble(); err != nil {
		return nil, err
	}
	if err := manager.Enemy.assemble(); err != nil {
		return nil, err
	}
	if err := manager.Scene.assemble(); err != nil {
		return nil, err
	}
	if err := manager.Exp.assemble(); err != nil {
		return nil, err
	}

	return manager, nil
}
