package gameconfig

const (
	FileCharacter  = "character.yaml"
	FileSkill      = "skill.yaml"
	FileEnemyGroup = "enemy.group.yaml"
	FileEnemyExp   = "enemy.exp.yaml"
	FileExp        = "exp.yaml"
	FileItem       = "item.yaml"
	FileItemWeapon = "item.weapon.yaml"
	FileReward     = "reward.yaml"
	FileTask       = "task.yaml"
	FileAI         = "ai.yaml"
	FilePet        = "pet.yaml"
	DirScene       = "scene"
)

var GGameConfig *Manager

type Manager struct {
	// Skill 是 skill.yaml 的统一技能配置, 用于校验角色和宠物的战斗技能输入.
	Skill *SkillConfig
	// AI 是 ai.yaml 的共享战斗AI配置, 由宠物模板通过ID引用.
	AI *AIConfig
	// Pet 是 pet.yaml 的宠物业务数值配置, 用于玩家宠物和敌人组引用的基础模板查询.
	Pet *PetConfig
	// Character 是 character.yaml 的角色资源索引配置, server 只消费角色ID和玩家可选角色标记.
	Character *CharacterConfig
	// Enemy 是 enemy.group.yaml 的敌人组配置, 用于生成战斗敌人并校验宠物模板引用.
	Enemy *EnemyGroupConfig
	// EnemyExp 是 enemy.exp.yaml 的敌人基础经验配置, 用于按敌人模板和等级生成初始 CHAR_EXP.
	EnemyExp *EnemyExpConfig
	// Scene 是 scene/*.yaml 汇总后的场景配置, 用于校验地图、阻挡和传送, 并按当前地图选择全地图遇敌规则.
	Scene *SceneConfig
	// Exp 是 exp.yaml 的等级经验配置, 用于按累计经验推导等级和下一等级门槛.
	Exp *ExpConfig
	// Item 合并 item.yaml 和 item.weapon.yaml 的道具与武器配置, 用于校验物品定义、使用效果和资源引用格式.
	Item *ItemConfig
	// Reward 是 reward.yaml 的任务奖励包配置, 当前仅包含道具和数量.
	Reward *RewardConfig
	// Task 是 task.yaml 的运行任务配置, 用于接取、推进、提交和领取步骤奖励.
	Task *TaskConfig
}
