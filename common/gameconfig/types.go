package gameconfig

const (
	FileCharacter  = "character.yaml"
	FileSkill      = "skill.yaml"
	FileEnemyGroup = "enemy.group.yaml"
	FileEnemyExp   = "enemy.exp.yaml"
	FileExp        = "exp.yaml"
	FileItem       = "item.yaml"
	FilePet        = "pet.yaml"
	FileScene      = "scene.yaml"
)

var GGameConfig *Manager

type Manager struct {
	// Skill 是 skill.yaml 的统一技能配置, 用于校验角色和宠物的战斗技能输入.
	Skill *SkillConfig
	// Pet 是 pet.yaml 的宠物业务数值配置, 用于玩家宠物和敌人组引用的基础模板查询.
	Pet *PetConfig
	// Character 是 character.yaml 的角色资源索引配置, server 只消费角色ID和玩家可选角色标记.
	Character *CharacterConfig
	// Enemy 是 enemy.group.yaml 的敌人组配置, 用于生成战斗敌人并校验宠物模板引用.
	Enemy *EnemyGroupConfig
	// EnemyExp 是 enemy.exp.yaml 的敌人基础经验配置, 用于按敌人模板和等级生成初始 CHAR_EXP.
	EnemyExp *EnemyExpConfig
	// Scene 是 scene.yaml 的场景配置, 用于校验角色所在场景并按场景权重选择敌人组.
	Scene *SceneConfig
	// Exp 是 exp.yaml 的等级经验配置, 用于按累计经验推导等级和下一等级门槛.
	Exp *ExpConfig
	// Item 是 item.yaml 的分组道具和装备配置, 用于校验物品定义、使用效果和资源引用格式.
	Item *ItemConfig
}
