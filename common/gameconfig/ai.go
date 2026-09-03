package gameconfig

import (
	"math"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type AIConfig struct {
	*xmap.MapMgr[uint32, *BattleAIEntry]
}

type BattleAITargetScope string

const (
	BattleAITargetScopeAllOpponents     BattleAITargetScope = "allOpponents"
	BattleAITargetScopePlayerCharacters BattleAITargetScope = "playerCharacters"
	BattleAITargetScopePlayerPets       BattleAITargetScope = "playerPets"
	BattleAITargetScopePartyLeader      BattleAITargetScope = "partyLeader"
)

const (
	BattleSkillIDAttack  uint32 = 8_000_001
	BattleSkillIDDefense uint32 = 8_000_002
	BattleSkillIDEscape  uint32 = 8_000_003
	BattleSkillIDCapture uint32 = 8_000_004
)

type BattleAITargetSelection string

const (
	BattleAITargetSelectionRandom          BattleAITargetSelection = "random"
	BattleAITargetSelectionHighestHP       BattleAITargetSelection = "highestHp"
	BattleAITargetSelectionLowestHP        BattleAITargetSelection = "lowestHp"
	BattleAITargetSelectionHighestAttack   BattleAITargetSelection = "highestAttack"
	BattleAITargetSelectionHighestAgility  BattleAITargetSelection = "highestAgility"
	BattleAITargetSelectionLowestAgility   BattleAITargetSelection = "lowestAgility"
	BattleAITargetSelectionElementalSubdue BattleAITargetSelection = "elementalSubdue"
)

type BattleAIEntry struct {
	// ID 来自 ai[].id, 由 enemy.group.yaml 的 enemies[].battleAI 引用.
	ID *uint32 `yaml:"id"`
	// Skills 按顺序保存敌人的战斗技能及相对权重, 不引用宠物出生技能槽.
	Skills []BattleAISkillEntry `yaml:"skills"`
	// TargetScope 限制需要通过AI选敌方目标的技能候选范围.
	TargetScope *BattleAITargetScope `yaml:"targetScope"`
	// TargetSelection 定义在候选目标中选择最终目标的方式.
	TargetSelection *BattleAITargetSelection `yaml:"targetSelection"`
	// TargetRandomRollMax对应8.5 battle_ai.c的rn[0], 只允许非random目标策略配置.
	// 服务端先执行RAND(0, value): 结果为0时再随机选择候选, 其余结果使用策略最优目标.
	// 因此值1表示50%随机、50%最优, 值0表示每次都随机且仍保留两次RAND调用.
	TargetRandomRollMax *uint32 `yaml:"targetRandomRollMax"`
}

type BattleAISkillEntry struct {
	// ID 引用 skill.yaml; 攻击、防御、逃跑和特殊技能使用同一种配置.
	ID *uint32 `yaml:"id"`
	// Weight 是相对选择权重, 必须显式填写且在[1,2147483647]范围内; 不使用的技能不配置.
	Weight *uint32 `yaml:"weight"`
}

// UnmarshalYAML拒绝分离的旧权重字段, 避免新旧配置混用时部分权重被忽略.
func (p *BattleAIEntry) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index+1 < len(node.Content); index += 2 {
		switch key := node.Content[index].Value; key {
		case "attackWeight", "defenseWeight", "escapeWeight", "skillSlotWeights":
			return errors.Errorf("ai.yaml 不再允许 %s, 请使用 skills[].id 和 skills[].weight", key)
		}
	}
	type battleAIEntry BattleAIEntry
	return node.Decode((*battleAIEntry)(p))
}

func newAIConfig() *AIConfig {
	return &AIConfig{
		MapMgr: xmap.NewMapMgr[uint32, *BattleAIEntry](),
	}
}

func (p *AIConfig) load(dir string) error {
	var root struct {
		AI []*BattleAIEntry `yaml:"ai"`
	}
	if err := loadYAMLFile(dir, FileAI, &root); err != nil {
		return err
	}
	if len(root.AI) == 0 {
		return errors.Errorf("AI配置 ai 段不能为空 %v", xruntime.Location())
	}
	for _, ai := range root.AI {
		if ai == nil || ai.ID == nil {
			return errors.Errorf("AI配置缺少 id %v", xruntime.Location())
		}
		if *ai.ID == 0 {
			return errors.Errorf("AI配置 id 必须大于0 %v", xruntime.Location())
		}
		if err := ai.check(*ai.ID); err != nil {
			return err
		}
		if !p.AddIfNotExist(*ai.ID, ai) {
			return errors.Errorf("AI配置ID重复: %d %v", *ai.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *AIConfig) check() error {
	var checkErr error
	p.Foreach(func(aiID uint32, ai *BattleAIEntry) bool {
		for _, skill := range ai.Skills {
			if GGameConfig.Skill == nil || !GGameConfig.Skill.IsExist(*skill.ID) {
				checkErr = errors.Errorf("AI引用了未定义技能: ai:%d skill:%d %v", aiID, *skill.ID, xruntime.Location())
				return false
			}
		}
		return true
	})
	return checkErr
}

func (p *AIConfig) assemble() error {
	return nil
}

// check只校验AI本身的技能、权重和目标策略. 技能定义引用由AIConfig.check
// 校验, 敌人组的AI引用由EnemyGroupConfig.check校验.
func (p *BattleAIEntry) check(aiID uint32) error {
	if len(p.Skills) == 0 {
		return errors.Errorf("AI配置 skills 不能为空: ai:%d %v", aiID, xruntime.Location())
	}
	weightSum := uint64(0)
	skillIDs := make(map[uint32]struct{}, len(p.Skills))
	for index, skill := range p.Skills {
		if skill.ID == nil || !isSkillID(*skill.ID) {
			return errors.Errorf("AI配置技能ID非法: ai:%d index:%d %v", aiID, index, xruntime.Location())
		}
		if _, exists := skillIDs[*skill.ID]; exists {
			return errors.Errorf("AI配置技能ID重复: ai:%d skill:%d %v", aiID, *skill.ID, xruntime.Location())
		}
		skillIDs[*skill.ID] = struct{}{}
		if skill.Weight == nil || *skill.Weight == 0 || *skill.Weight > math.MaxInt32 {
			return errors.Errorf("AI配置技能weight必须在[1,2147483647]: ai:%d skill:%d %v", aiID, *skill.ID, xruntime.Location())
		}
		weightSum += uint64(*skill.Weight)
	}
	if weightSum > math.MaxInt32 {
		return errors.Errorf("AI配置动作权重总和必须在[1,2147483647]: ai:%d total:%d %v",
			aiID, weightSum, xruntime.Location())
	}
	if p.TargetScope == nil || (*p.TargetScope != BattleAITargetScopeAllOpponents &&
		*p.TargetScope != BattleAITargetScopePlayerCharacters &&
		*p.TargetScope != BattleAITargetScopePlayerPets &&
		*p.TargetScope != BattleAITargetScopePartyLeader) {
		return errors.Errorf("AI配置 targetScope 非法: ai:%d value:%v %v",
			aiID, p.TargetScope, xruntime.Location())
	}
	if p.TargetSelection == nil || (*p.TargetSelection != BattleAITargetSelectionRandom &&
		*p.TargetSelection != BattleAITargetSelectionHighestHP &&
		*p.TargetSelection != BattleAITargetSelectionLowestHP &&
		*p.TargetSelection != BattleAITargetSelectionHighestAttack &&
		*p.TargetSelection != BattleAITargetSelectionHighestAgility &&
		*p.TargetSelection != BattleAITargetSelectionLowestAgility &&
		*p.TargetSelection != BattleAITargetSelectionElementalSubdue) {
		return errors.Errorf("AI配置 targetSelection 非法: ai:%d value:%v %v",
			aiID, p.TargetSelection, xruntime.Location())
	}
	if *p.TargetSelection == BattleAITargetSelectionRandom {
		if p.TargetRandomRollMax != nil {
			return errors.Errorf("AI配置 random目标策略不能配置 targetRandomRollMax: ai:%d value:%d %v",
				aiID, *p.TargetRandomRollMax, xruntime.Location())
		}
		return nil
	}
	if p.TargetRandomRollMax == nil {
		return errors.Errorf("AI配置非random目标策略缺少 targetRandomRollMax: ai:%d selection:%s %v",
			aiID, *p.TargetSelection, xruntime.Location())
	}
	if *p.TargetRandomRollMax > math.MaxInt32 {
		return errors.Errorf("AI配置 targetRandomRollMax 超出[0,2147483647]: ai:%d value:%d %v",
			aiID, *p.TargetRandomRollMax, xruntime.Location())
	}
	return nil
}
