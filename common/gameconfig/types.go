package gameconfig

const (
	FileCharacter      = "character.yaml"
	FileCharacterSkill = "character.skill.yaml"
	FileEnemyGroup     = "enemy.group.yaml"
	FileEnemyExp       = "enemy.exp.yaml"
	FileExp            = "exp.yaml"
	FileItem           = "item.yaml"
	FilePetSkill       = "pet.skill.yaml"
	FilePet            = "pet.yaml"
	FileScene          = "scene.yaml"
)

var GGameConfig *Manager

type Manager struct {
	// CharacterSkill 是 character.skill.yaml 的角色技能配置, 用于校验角色战斗技能输入.
	CharacterSkill *CharacterSkillConfig
	// PetSkill 是 pet.skill.yaml 的宠物技能配置, 用于校验宠物技能槽位引用和按技能ID查询技能描述.
	PetSkill *PetSkillConfig
	// Pet 是 pet.yaml 的宠物业务数值配置, 用于宠物模板查询和敌人组宠物ID引用校验.
	Pet *PetConfig
	// Character 是 character.yaml 的角色资源索引配置, server 只消费角色ID和玩家可选角色标记.
	Character *CharacterConfig
	// Enemy 是 enemy.group.yaml 的敌人组配置, 用于生成战斗敌人模板和校验宠物模板引用.
	Enemy *EnemyGroupConfig
	// EnemyExp 是 enemy.exp.yaml 的敌人基础经验配置, 用于按敌人模板和等级生成初始 CHAR_EXP.
	EnemyExp *EnemyExpConfig
	// Scene 是 scene.yaml 的场景配置, 用于校验角色所在场景并按场景权重选择敌人组.
	Scene *SceneConfig
	// Exp 是 exp.yaml 的等级经验配置, 用于按累计经验推导等级和下一等级门槛.
	Exp *ExpConfig
	// Item 是 item.yaml 的可堆叠道具配置, 用于校验道具数量与使用效果.
	Item *ItemConfig
}
