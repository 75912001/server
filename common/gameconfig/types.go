package gameconfig

const (
	FileCharacter               = "character.yaml"
	FileSkill                   = "技能.yaml"
	FileEnemyGroup              = "enemy.group.yaml"
	FileEnemyExp                = "enemy.exp.yaml"
	FileExp                     = "exp.yaml"
	FileItem                    = "item.yaml"
	FileItemMaterial            = "道具.素材.yaml"
	FileItemCurrency            = "item.currency.yaml"
	FileItemSynthesis           = "item.synthesis.yaml"
	FileItemTiangong            = "道具.天工司.yaml"
	FileItemWeaponClaw          = "道具.武器.爪.yaml"
	FileItemWeaponAxe           = "道具.武器.斧.yaml"
	FileItemWeaponStaff         = "道具.武器.棍.yaml"
	FileItemWeaponSpear         = "道具.武器.枪.yaml"
	FileItemWeaponBow           = "道具.武器.弓.yaml"
	FileItemWeaponBoomerang     = "道具.武器.回旋镖.yaml"
	FileItemWeaponThrowingAxe   = "道具.武器.投掷斧.yaml"
	FileItemWeaponThrowingStone = "道具.武器.投掷石.yaml"
	FileItemEquipmentChest      = "item.equipment.chest.yaml"
	FileItemEquipmentHelmet     = "item.equipment.helmet.yaml"
	FileItemEquipmentShield     = "item.equipment.shield.yaml"
	FileItemEquipmentGloves     = "item.equipment.gloves.yaml"
	FileItemEquipmentBelt       = "item.equipment.belt.yaml"
	FileItemEquipmentBoots      = "item.equipment.boots.yaml"
	FileItemEquipmentAccessory  = "item.equipment.accessory.yaml"
	FileReward                  = "reward.yaml"
	FileTask                    = "task.yaml"
	FileAI                      = "ai.yaml"
	FileGrowthAttribute         = "growth.attribute.yaml"
	FilePet                     = "pet.yaml"
	FileStore                   = "商店.yaml"
	DirScene                    = "scene"
)

var FileItemWeapons = []string{
	FileItemWeaponClaw,
	FileItemWeaponAxe,
	FileItemWeaponStaff,
	FileItemWeaponSpear,
	FileItemWeaponBow,
	FileItemWeaponBoomerang,
	FileItemWeaponThrowingAxe,
	FileItemWeaponThrowingStone,
}

var FileItemEquipments = []string{
	FileItemEquipmentChest,
	FileItemEquipmentHelmet,
	FileItemEquipmentShield,
	FileItemEquipmentGloves,
	FileItemEquipmentBelt,
	FileItemEquipmentBoots,
	FileItemEquipmentAccessory,
}

var GGameConfig *Manager

type Manager struct {
	// Skill 是 技能.yaml 的统一技能配置, 用于校验角色和宠物的战斗技能输入.
	Skill *SkillConfig
	// AI 是 ai.yaml 的共享战斗AI配置, 由宠物模板通过ID引用.
	AI *AIConfig
	// GrowthAttribute 是 growth.attribute.yaml 的共享成长属性配置, 为宠物实例和敌人提供权威数值.
	GrowthAttribute *GrowthAttributeConfig
	// Pet 是 pet.yaml 的宠物外观和出生技能配置, 通过ID引用默认成长属性.
	Pet *PetConfig
	// Character 是 character.yaml 的角色资源索引配置, 保存角色ID、玩家可选标记和骑乘宠物权限集合.
	Character *CharacterConfig
	// Enemy 是 enemy.group.yaml 的敌人组配置, 用于组合外观、成长属性和战斗AI.
	Enemy *EnemyGroupConfig
	// EnemyExp 是 enemy.exp.yaml 的敌人基础经验配置, 用于按敌人模板和等级生成初始 CHAR_EXP.
	EnemyExp *EnemyExpConfig
	// Scene 是 scene/*.yaml 汇总后的场景配置, 用于校验地图、阻挡和传送, 并按当前地图选择全地图遇敌规则.
	Scene *SceneConfig
	// Exp 是 exp.yaml 的等级经验配置, 用于按累计经验推导等级和下一等级门槛.
	Exp *ExpConfig
	// Item 合并普通道具、素材、货币、八类武器和七类装备配置, 用于校验物品定义、使用效果和资源引用格式.
	Item *ItemConfig
	// Tiangong 是宠物加工的基础武器配方, 不包含附加技能和元素属性.
	Tiangong *TiangongConfig
	// Reward 是 reward.yaml 的任务奖励包配置, 支持普通道具、装备实例和宠物实例.
	Reward *RewardConfig
	// Task 是 task.yaml 的运行任务配置, 用于接取、推进、宠物或道具提交、循环任务和步骤奖励.
	Task *TaskConfig
	// Store 是交换系统运行配置, 商店.yaml 内按 type 分块维护条目列表.
	Store *StoreConfig
}
