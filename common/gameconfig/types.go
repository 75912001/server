package gameconfig

import pb "server/proto/pb"

const (
	FileCharacter  = "character.yaml"
	FileEnemyGroup = "enemy.group.yaml"
	FileExp        = "exp.yaml"
	FilePetSkill   = "pet.skill.yaml"
	FilePet        = "pet.yaml"
)

type Manager struct {
	// PetSkill 是 pet.skill.yaml 的宠物技能配置, 用于校验宠物技能槽位引用和按技能ID查询技能描述.
	PetSkill *PetSkillConfig
	// Pet 是 pet.yaml 的宠物业务数值配置, 用于宠物模板查询和敌人组宠物ID引用校验.
	Pet *PetConfig
	// Character 是 character.yaml 的角色资源索引配置, server 只消费角色ID和玩家可选角色标记.
	Character *CharacterConfig
	// Enemy 是 enemy.group.yaml 的敌人组配置, 用于生成战斗敌人模板和校验宠物模板引用.
	Enemy *EnemyGroupConfig
	// Exp 是 exp.yaml 的等级经验配置, 用于按累计经验推导等级和下一等级门槛.
	Exp *ExpConfig
}

type PetSkillConfig struct {
	byID map[int]*PetSkillEntry
	ids  []int
}

type PetSkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议宠物技能资源ID段内, 并且在 pet.skill.yaml 内唯一.
	ID int
	// Name 来自 skill[].name, 保留技能显示名称文本, server 不解析客户端资源.
	Name string
	// Description 来自 skill[].description, 保留技能说明文本, 可用于查询, 日志或后续业务展示下发.
	Description string
}

type PetConfig struct {
	byID map[int]*PetEntry
	ids  []int
}

type PetEntry struct {
	// ID 来自 pet[].id, 必须处于协议宠物资源ID段内, 并且会被 enemy.group.yaml 的 enemies[].id 引用.
	ID int
	// Rarity 来自 pet[].rarity, 使用协议 PetRarity 的整数值, 当前范围为普通到神话.
	Rarity int
	// Elemental 来自 pet[].elemental, key 转为协议元素类型, 值范围[0,10], 总和必须为10.
	Elemental map[pb.AssetElemental]int
	// Attribute 来自 pet[].attribute, 保存宠物抗性和战斗附加属性, 字段允许负数.
	Attribute PetAttributeEntry
	// Growth 来自 pet[].growth, 保存宠物生成和升级时使用的基础成长参数.
	Growth PetGrowthEntry
	// SkillSlots 来自 pet[].skill, 按配置顺序保存技能槽位; 0 表示空槽, 非0值必须引用 pet.skill.yaml 中存在的技能ID.
	SkillSlots []int
}

type PetAttributeEntry struct {
	// PoisonResist 来自 attribute.poisonResist, 表示毒抗性修正值.
	PoisonResist int
	// ParalysisResist 来自 attribute.paralysisResist, 表示麻痹抗性修正值.
	ParalysisResist int
	// SleepResist 来自 attribute.sleepResist, 表示睡眠抗性修正值.
	SleepResist int
	// StoneResist 来自 attribute.stoneResist, 表示石化抗性修正值.
	StoneResist int
	// DrunkResist 来自 attribute.drunkResist, 表示酒醉抗性修正值.
	DrunkResist int
	// ConfusionResist 来自 attribute.confusionResist, 表示混乱抗性修正值.
	ConfusionResist int
	// Critical 来自 attribute.critical, 表示暴击相关修正值.
	Critical int
	// Counter 来自 attribute.counter, 表示反击相关修正值.
	Counter int
}

type PetGrowthEntry struct {
	// InitNum 来自 growth.initNum, 表示初始系数, 参与 1 级宠物初始四维计算.
	InitNum int
	// LvupPointSource 来自 growth.lvupPointSource, 表示原始升级成长点字段, 必须大于0.
	LvupPointSource float64
	// BaseVital 来自 growth.baseVital, 表示宠物模板固定基础体力值, 加随机最小偏移后必须仍大于0.
	BaseVital int
	// BaseStr 来自 growth.baseStr, 表示宠物模板固定基础腕力/攻击值, 加随机最小偏移后必须仍大于0.
	BaseStr int
	// BaseTough 来自 growth.baseTough, 表示宠物模板固定基础耐力/防御值, 加随机最小偏移后必须仍大于0.
	BaseTough int
	// BaseDex 来自 growth.baseDex, 表示宠物模板固定基础速度/敏捷值, 加随机最小偏移后必须仍大于0.
	BaseDex int
}

type CharacterConfig struct {
	byID map[int]*CharacterEntry
	ids  []int
}

type CharacterEntry struct {
	// ID 来自 character[].id, 必须处于协议角色资源ID段内, 并且在 character.yaml 内唯一.
	ID int
	// IsRole 来自 character[].isRole, 标记该角色资源是否可作为玩家角色; 缺省时按 false 处理.
	IsRole bool
}

type EnemyGroupConfig struct {
	byID map[int]*EnemyGroupEntry
	ids  []int
}

type EnemyGroupEntry struct {
	// ID 来自 enemyGroups[].id, 必须为正数, 并且在 enemy.group.yaml 内唯一.
	ID int
	// Name 来自 enemyGroups[].name, 表示敌人组名称, 主要用于配置识别, 日志和排障.
	Name string
	// IsBoss 来自 enemyGroups[].isBoss, 缺省为 false; Boss 组按固定 enemies 顺序出怪, 不使用普通组随机规则.
	IsBoss bool
	// CountRange 来自 enemyGroups[].countRange, 表示普通敌人组出怪数量范围; Boss 组不允许配置.
	CountRange IntRange
	// LevelRange 来自 enemyGroups[].levelRange, 表示普通敌人组随机等级范围; 与 RoleLevelOffset 至少配置一个, Boss 组不允许配置.
	LevelRange IntRange
	// RoleLevelOffset 来自 enemyGroups[].roleLevelOffset, 表示基于玩家等级的随机偏移范围; 与 LevelRange 至少配置一个, Boss 组不允许配置.
	RoleLevelOffset IntRange
	// Captured 来自 enemyGroups[].captured, 表示普通敌人组是否允许捕获, 缺省为 true; Boss 组固定为 false.
	Captured bool
	// BabyRate 来自 enemyGroups[].babyRate, 表示每只敌人成为 1 级宠物宝宝的十万分率, 缺省为0; Boss 组不允许配置.
	BabyRate int
	// Enemies 来自 enemyGroups[].enemies, 保存敌人模板列表, 每个敌人ID必须引用 pet.yaml 中存在的宠物ID.
	Enemies []EnemyEntry
}

type EnemyEntry struct {
	// ID 来自 enemies[].id, 表示作为敌人模板的宠物ID, 必须能在 pet.yaml 中找到.
	ID int
	// Weight 来自 enemies[].weight, 表示普通敌人组随机选择权重, 缺省为0且代表必定出现; Boss 组不允许配置.
	Weight int
	// Level 来自 enemies[].level, 表示指定敌人等级; Boss 组必填, 普通组可选, 值必须处于协议等级范围.
	Level int
}

type IntRange struct {
	// Min 表示闭区间最小值, 由 YAML 中二元数组的第一个元素解析得到.
	Min int
	// Max 表示闭区间最大值, 由 YAML 中二元数组的第二个元素解析得到, 且必须大于等于 Min.
	Max int
}

type ExpConfig struct {
	byLevel map[int]*LevelEntry
	levels  []*LevelEntry
	max     int
}

type LevelEntry struct {
	// Level 来自 exp.yaml 的 levels map key, 必须完整覆盖协议等级范围且连续.
	Level int
	// MinExp 是 server 在加载 exp.yaml 时派生的本等级最小累计经验, 1 级固定为0, 其他等级为上一等级 MaxExp+1.
	MinExp int
	// MaxExp 来自 levels.<level>.max, 表示本等级最大累计经验, 必须非负并随等级严格递增.
	MaxExp int
}

func (p *PetSkillConfig) HasID(id int) bool {
	if p == nil {
		return false
	}
	_, ok := p.byID[id]
	return ok
}

func (p *PetSkillConfig) GetByID(id int) *PetSkillEntry {
	if p == nil {
		return nil
	}
	return p.byID[id]
}

func (p *PetSkillConfig) IDs() []int {
	if p == nil {
		return nil
	}
	return append([]int(nil), p.ids...)
}

func (p *PetConfig) HasID(id int) bool {
	if p == nil {
		return false
	}
	_, ok := p.byID[id]
	return ok
}

func (p *PetConfig) GetByID(id int) *PetEntry {
	if p == nil {
		return nil
	}
	return p.byID[id]
}

func (p *PetConfig) IDs() []int {
	if p == nil {
		return nil
	}
	return append([]int(nil), p.ids...)
}

func (c *CharacterConfig) GetByID(id int) *CharacterEntry {
	if c == nil {
		return nil
	}
	return c.byID[id]
}

func (c *CharacterConfig) IDs() []int {
	if c == nil {
		return nil
	}
	return append([]int(nil), c.ids...)
}

func (e *EnemyGroupConfig) GetByID(id int) *EnemyGroupEntry {
	if e == nil {
		return nil
	}
	return e.byID[id]
}

func (e *EnemyGroupConfig) IDs() []int {
	if e == nil {
		return nil
	}
	return append([]int(nil), e.ids...)
}

func (e *ExpConfig) GetLevel(totalExp int) (int, error) {
	if totalExp < 0 {
		return 0, configError("总经验不能为负数: %d", totalExp)
	}
	if e == nil || len(e.levels) == 0 {
		return 0, configError("经验配置尚未加载")
	}
	for _, entry := range e.levels {
		if totalExp <= entry.MaxExp {
			return entry.Level, nil
		}
	}
	return e.max, nil
}

func (e *ExpConfig) GetNextLevelTotalExp(totalExp int) (int, error) {
	level, err := e.GetLevel(totalExp)
	if err != nil {
		return 0, err
	}
	if level >= e.max {
		return -1, nil
	}
	next := e.byLevel[level+1]
	return next.MinExp, nil
}

func (e *ExpConfig) GetLevelMinExp(level int) (int, error) {
	if e == nil || len(e.levels) == 0 {
		return 0, configError("经验配置尚未加载")
	}
	entry := e.byLevel[level]
	if entry == nil {
		return 0, configError("经验等级不存在: %d", level)
	}
	return entry.MinExp, nil
}

func (e *ExpConfig) IsMaxLevel(totalExp int) (bool, error) {
	level, err := e.GetLevel(totalExp)
	if err != nil {
		return false, err
	}
	return level >= e.max, nil
}
