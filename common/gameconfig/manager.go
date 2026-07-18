package gameconfig

func Load(dir string) error {
	GGameConfig = &Manager{
		CharacterSkill: newCharacterSkillConfig(),
		PetSkill:       newPetSkillConfig(),
		Pet:            newPetConfig(),
		Character:      newCharacterConfig(),
		Enemy:          newEnemyGroupConfig(),
		EnemyExp:       newEnemyExpConfig(),
		Scene:          newSceneConfig(),
		Exp:            newExpConfig(),
	}
	if err := GGameConfig.CharacterSkill.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.PetSkill.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.Pet.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.Character.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.Enemy.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.EnemyExp.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.Scene.load(dir); err != nil {
		return err
	}
	if err := GGameConfig.Exp.load(dir); err != nil {
		return err
	}

	if err := GGameConfig.CharacterSkill.check(); err != nil {
		return err
	}
	if err := GGameConfig.PetSkill.check(); err != nil {
		return err
	}
	if err := GGameConfig.Pet.check(); err != nil {
		return err
	}
	if err := GGameConfig.Character.check(); err != nil {
		return err
	}
	if err := GGameConfig.Enemy.check(); err != nil {
		return err
	}
	if err := GGameConfig.EnemyExp.check(); err != nil {
		return err
	}
	if err := GGameConfig.Scene.check(); err != nil {
		return err
	}
	if err := GGameConfig.Exp.check(); err != nil {
		return err
	}

	if err := GGameConfig.CharacterSkill.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.PetSkill.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.Pet.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.Character.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.Enemy.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.EnemyExp.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.Scene.assemble(); err != nil {
		return err
	}
	if err := GGameConfig.Exp.assemble(); err != nil {
		return err
	}

	return nil
}
