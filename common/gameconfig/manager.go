package gameconfig

func Load(dir string) (err error) {
	GGameConfig = &Manager{
		Skill:     newSkillConfig(),
		Pet:       newPetConfig(),
		Character: newCharacterConfig(),
		Enemy:     newEnemyGroupConfig(),
		EnemyExp:  newEnemyExpConfig(),
		Scene:     newSceneConfig(),
		Exp:       newExpConfig(),
		Item:      newItemConfig(),
	}
	if err := GGameConfig.Skill.load(dir); err != nil {
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
	if err := GGameConfig.Item.load(dir); err != nil {
		return err
	}

	if err := GGameConfig.Skill.check(); err != nil {
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
	if err := GGameConfig.Item.check(); err != nil {
		return err
	}

	if err := GGameConfig.Skill.assemble(); err != nil {
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
	if err := GGameConfig.Item.assemble(); err != nil {
		return err
	}

	return nil
}
