package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"server/common/gameconfig"
	commonpet "server/common/pet"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xtimer "github.com/75912001/xlib/timer"
	xutil "github.com/75912001/xlib/util"
	"google.golang.org/protobuf/proto"
)

const (
	// 自动遇敌采用一次性定时器. 每次触发后由客户端战斗流程完成消息决定是否注册下一次遭遇.
	autoEncounterInterval = 5 * time.Second
	// 每回合最多等待玩家操作 100 秒, 超时后服务端为尚未操作的存活单位补齐默认动作.
	combatRoundDuration = 100 * time.Second

	// 发起方阵营内角色与宠物使用不同的位置区间, 与协议中的阵营站位约定保持一致.
	initiatorCharacterPosition = 0
	initiatorPetPosition       = 5
	// 8.5每个阵营有前后两排, 每排固定5个Entry.
	combatCampRowPositionCount = 5
	// ENEMY_createEnemy在四项模板基础值完成独立偏移后, 固定再随机分配10点.
	// 每点只会增加体力、腕力、耐力或速度中的一项, 不允许配置改变这个旧服常量.
	combatPVEEnemyInitialBonusPointCount = 10
	// GETITEM_MAX是每个玩家Entry在BATTLE_AddExpItem阶段最多暂存的战利品数.
	// 该上限属于战斗内临时槽, 与角色背包30条目的持久化上限不是同一个概念.
	combatPVEGetItemMax = 3

	// 战斗技能ID只来自skill.yaml. 8000004至8000007虽然是有效配置,
	// 但当前online没有对应处理器, 请求会返回不支持技能错误.
	combatSkillAttack     = 8000001
	combatSkillDefense    = 8000002
	combatSkillEscape     = 8000003
	combatSkillStandby    = 8100000
	combatSkillGuardBreak = 8100003
)

// combatTargetAttributeSnapshot保存敌方AI目标选择需要的原始腕力和速度.
//
// CombatUnit只保存合成后的攻击和敏捷, 无法无损反推出原始四维. 建房时按稳定
// 单位键暂存这两个值并复制到运行态, 不进入协议或持久化数据.
type combatTargetAttributeSnapshot struct {
	strength  uint64
	dexterity uint64
}

// combatUnitRuntimeState将不可变的单位开战数据与战斗运行态分离.
type combatUnitRuntimeState struct {
	// unit指向battleStart中的只读单位快照.
	unit *pb.CombatUnit

	// PVE敌方身份和奖励在建房时冻结, 避免战斗中热更新配置改变结算.
	pveEnemy          bool
	enemyExperience   uint32
	enemyDuelPoint    int32
	enemyDropAssetIDs []uint32

	// 玩家侧累计结果只在战斗结束时持久化.
	battleDropAssetIDs    []uint32
	characterDuelPoint    uint32
	defeatProfitProcessed bool
	battleExperience      uint64
	battleDuelPoint       int64

	// 当前生命、魔法和在场状态使用运行态保存, 不修改已发送的开战快照.
	hp      uint64
	maxHP   uint64
	mp      uint64
	maxMP   uint64
	alive   bool
	escaped bool
	guard   bool

	// 原始腕力和速度只用于敌方AI按8.5规则选择目标.
	rawStrength  uint64
	rawDexterity uint64

	// 基础物理、逃跑和击飞结算所需的跨回合状态.
	escapeAttempts          uint32
	overkillDamage          uint64
	ultimateKnockbackImmune bool
	inanimate               bool
	rare                    uint32
	hitModifier             int64
	dodgeModifier           int64
	criticalModifier        int64
	counterModifier         int64
}

// applyPetBattleTraits把宠物模板中不进入协议的8.5固有战斗特性冻结到单位运行态.
//
// 玩家宠物和敌方宠物都按PetId读取同一份服务器配置. 角色没有PetId, 因而保持
// 零值. 配置只在建房时读取一次, 后续即使热更新PetEntry也不会改变进行中的战斗;
// 客户端提交的技能ID、目标或表现资源同样不能覆盖这些特性.
func applyPetBattleTraits(state *combatUnitRuntimeState, pet *gameconfig.PetEntry) {
	if state == nil || pet == nil || pet.Attribute == nil {
		return
	}
	if pet.Attribute.Rare != nil {
		state.rare = *pet.Attribute.Rare
	}
	state.ultimateKnockbackImmune = pet.Attribute.UltimateKnockbackImmune
	state.inanimate = pet.Attribute.Inanimate
}

// onAutoEncounterSetReq 按角色 UUID 设置自动遇敌开关, 开启前校验目标角色和场景状态, 战斗宠物可选.
func (p *Account) onAutoEncounterSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	// 客户端包体属于不可信边界, 反序列化失败直接返回参数错误, 不改变当前开关和定时器状态.
	var req pb.CombatAutoEncounterSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	if characterUUID == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	// 只有"开启"需要预先检查战斗条件. "关闭"必须始终可执行, 便于客户端及时停止后续遭遇.
	if req.GetEnabled() {
		characterRecord, _, err := character.currentBattleCharacterAndPet()
		if err != nil {
			p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
		sceneEntry := p.getCharacterScene(characterRecord)
		if nil == sceneEntry {
			p.sendClientErr(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
	}

	character.autoEncounterEnabled = req.GetEnabled()
	if character.autoEncounterEnabled {
		// 已加入 CombatRoom 时不注册新任务; 客户端完成战斗表现并回到非战斗场景后再请求启动计时.
		if character.combatRoom == nil {
			character.restartAutoEncounterTimer(gateway)
		}
	} else {
		character.clearAutoEncounterTimer()
	}

	p.sendClientRes(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.Success.Code(),
		&pb.CombatAutoEncounterSetRes{
			Enabled:           character.autoEncounterEnabled,
			ServerTimestampMs: time.Now().UnixMilli(),
			CharacterUuid:     characterUUID,
		},
	)
}

// onCombatRoundActionReq 校验账号边界和目标角色后, 将回合动作投递给 CombatRoom actor.
func (p *Account) onCombatRoundActionReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CombatRoundActionReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	unitKey := req.GetUnitKey()
	if unitKey == nil || unitKey.GetAid() != p.aid || unitKey.GetCharacterUuid() == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := unitKey.GetCharacterUuid()
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	character.combatRoom.PostRoundAction(combatRoomParticipantKey{
		aid:           p.aid,
		characterUUID: characterUUID,
	}, gateway, &req)
}

// onCombatRoundActionReq 在房间 actor 内校验并记录玩家动作, 全部可控单位就绪后立即结算回合.
func (r *CombatRoom) onCombatRoundActionReq(key combatRoomParticipantKey, gateway *Gateway, req *pb.CombatRoundActionReq) {
	participant := r.participant(key)
	if participant == nil || participant.account == nil {
		return
	}
	p := participant.account
	characterUUID := participant.key.characterUUID
	// 同时校验战斗 ID 和回合号, 防止网络延迟或客户端重试把旧动作写入当前回合.
	if req.GetBattleId() != r.battleID || req.GetRound() != r.round {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	var action *combatAction
	var err error
	// 服务端只接受开战时登记的角色和宠物单位键, 不信任客户端自行声明的可控单位身份.
	switch {
	case req.GetUnitKey().GetAid() != participant.key.aid || req.GetUnitKey().GetCharacterUuid() != characterUUID:
		err = fmt.Errorf("unit character uuid mismatch")
	case combatUnitKeyEqual(req.GetUnitKey(), participant.playerCharacter.GetKey()):
		if !r.isAlive(participant.playerCharacter.GetKey()) {
			err = fmt.Errorf("character is not alive")
		} else {
			action, err = r.characterCombatSkillAction(participant.playerCharacter, combatSkillInputFromRequest(req))
		}
	case participant.playerPet != nil && combatUnitKeyEqual(req.GetUnitKey(), participant.playerPet.GetKey()):
		if !r.isAlive(participant.playerPet.GetKey()) {
			err = fmt.Errorf("pet is not alive")
		} else {
			action, err = r.petCombatSkillAction(participant.playerPet, combatSkillInputFromRequest(req))
		}
	default:
		err = fmt.Errorf("unit is not controlled by player")
	}
	if err != nil {
		xlog.GLog.Warnf("invalid combat action aid:%d battle:%s round:%d err:%v", p.aid, req.GetBattleId(), req.GetRound(), err)
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	// 每个可控单位每回合只能提交一次. 动作写入后即锁定, 后续重复请求不会覆盖已确认的意图.
	actionKey := combatUnitKeyMapKey(action.unitKey)
	if _, ok := r.playerActions[actionKey]; ok {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	r.playerActions[actionKey] = action

	// 合法动作确认广播给房间内全部玩家, 客户端据此维护当前回合已提交单位集合.
	for _, participantKey := range r.participantOrder {
		targetParticipant := r.participant(participantKey)
		if targetParticipant == nil {
			continue
		}
		targetParticipant.account.sendClientRes(targetParticipant.gateway, uint32(pb.MsgID_CombatRoundActionRes_CMD), xerror.Success.Code(), &pb.CombatRoundActionRes{
			BattleId: r.battleID,
			Round:    r.round,
			UnitKey:  cloneCombatUnitKey(action.unitKey),
		})
	}
	// 全部存活的玩家单位均已提交后立即结算, 无需继续等待回合定时器.
	if r.playerActionsReady() {
		r.completeCombatRound(r.collectedPlayerActions())
	}
}

// combatSkillInput 是服务端内部技能输入, 隔离传输请求和敌方AI的参数来源.
type combatSkillInput struct {
	SkillId       uint32
	ArgTargetUnit *pb.CombatUnitKey
	ArgTargetCamp pb.CombatCamp
}

func combatSkillInputFromRequest(req *pb.CombatRoundActionReq) *combatSkillInput {
	if req == nil {
		return nil
	}
	return &combatSkillInput{
		SkillId:       req.GetSkillId(),
		ArgTargetUnit: req.GetArgTargetUnit(),
		ArgTargetCamp: req.GetArgTargetCamp(),
	}
}

func (input *combatSkillInput) GetSkillId() uint32 {
	if input == nil {
		return 0
	}
	return input.SkillId
}

func (input *combatSkillInput) GetArgTargetUnit() *pb.CombatUnitKey {
	if input == nil {
		return nil
	}
	return input.ArgTargetUnit
}

func (input *combatSkillInput) GetArgTargetCamp() pb.CombatCamp {
	if input == nil {
		return pb.CombatCamp_CombatCamp_Initiator
	}
	return input.ArgTargetCamp
}

func (r *CombatRoom) characterCombatSkillAction(unit *pb.CombatUnit, input *combatSkillInput) (*combatAction, error) {
	if unit == nil || input == nil {
		return nil, fmt.Errorf("character skill action is missing")
	}
	skillID := input.GetSkillId()
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Skill == nil || !gameconfig.GGameConfig.Skill.IsExist(skillID) {
		return nil, fmt.Errorf("combat skill is not configured: %d", skillID)
	}
	action := &combatAction{unitKey: cloneCombatUnitKey(unit.GetKey()), skillID: skillID}
	switch skillID {
	case combatSkillAttack:
		target, err := r.validOpponentTarget(input.GetArgTargetUnit(), unit.GetKey())
		if err != nil {
			return nil, err
		}
		action.kind = combatActionKindAttack
		action.targetKey = target
	case combatSkillDefense:
		action.kind = combatActionKindDefense
		action.targetKey = cloneCombatUnitKey(unit.GetKey())
	case combatSkillEscape:
		action.kind = combatActionKindEscape
	default:
		return nil, fmt.Errorf("unsupported character combat skill: %d", skillID)
	}
	return action, nil
}

// enemyPetCombatSkillAction将敌方AI选中的技能槽交给统一的宠物技能解析器.
func (r *CombatRoom) enemyPetCombatSkillAction(unit *pb.CombatUnit, skillID uint32, selectedTarget *pb.CombatUnitKey) (*combatAction, error) {
	if unit == nil || selectedTarget == nil {
		return nil, fmt.Errorf("enemy pet skill action is missing")
	}
	return r.petCombatSkillAction(unit, &combatSkillInput{
		SkillId:       skillID,
		ArgTargetUnit: cloneCombatUnitKey(selectedTarget),
	})
}

func (r *CombatRoom) petCombatSkillAction(unit *pb.CombatUnit, input *combatSkillInput) (*combatAction, error) {
	if unit == nil || input == nil {
		return nil, fmt.Errorf("pet skill action is missing")
	}
	skillID := input.GetSkillId()
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Skill == nil {
		return nil, fmt.Errorf("combat skill is not configured: %d", skillID)
	}
	skill := gameconfig.GGameConfig.Skill.Get(skillID)
	if skill == nil {
		return nil, fmt.Errorf("combat skill is not configured: %d", skillID)
	}
	if !petUnitOwnsConfiguredSkill(unit, skillID) {
		return nil, fmt.Errorf("pet does not own combat skill: pet:%d skill:%d", unit.GetPetId(), skillID)
	}

	action := &combatAction{unitKey: cloneCombatUnitKey(unit.GetKey()), skillID: skillID}
	switch {
	case skillID == combatSkillStandby:
		action.kind = combatActionKindStandby
	case skillID == combatSkillAttack:
		target, err := r.validOpponentTarget(input.GetArgTargetUnit(), unit.GetKey())
		if err != nil {
			return nil, err
		}
		action.kind = combatActionKindAttack
		action.targetKey = target
	case skillID == combatSkillDefense:
		action.kind = combatActionKindDefense
		action.targetKey = cloneCombatUnitKey(unit.GetKey())
	case skillID == combatSkillGuardBreak:
		target, err := r.validOpponentTarget(input.GetArgTargetUnit(), unit.GetKey())
		if err != nil {
			return nil, err
		}
		action.kind = combatActionKindGuardBreak
		action.targetKey = target
	case skill.ContinuationAttack != nil:
		if skill.ContinuationAttack.SegmentCount == nil {
			return nil, fmt.Errorf("continuation attack skill config is incomplete: %d", skillID)
		}
		target, err := r.validOpponentTarget(input.GetArgTargetUnit(), unit.GetKey())
		if err != nil {
			return nil, err
		}
		action.kind = combatActionKindContinuationAttack
		action.targetKey = target
		action.segmentCount = *skill.ContinuationAttack.SegmentCount
	default:
		return nil, fmt.Errorf("unsupported pet combat skill: %d", skillID)
	}
	return action, nil
}

// petUnitOwnsConfiguredSkill校验技能是否存在于该宠物物种的技能槽.
//
// 当前PetRecord没有单独持久化学习技能, 因此以CombatUnit.pet_id对应的
// pet.yaml skill列表为权威来源. 本函数只做本地配置查询, 不修改技能槽,
// 也不为未配置技能的宠物提供默认行为.
func petUnitOwnsConfiguredSkill(unit *pb.CombatUnit, skillID uint32) bool {
	if unit == nil || unit.GetPetId() == 0 || skillID == 0 ||
		gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Pet == nil {
		return false
	}
	pet := gameconfig.GGameConfig.Pet.Get(unit.GetPetId())
	if pet == nil {
		return false
	}
	for _, configuredSkillID := range pet.SkillSlots {
		if configuredSkillID == skillID {
			return true
		}
	}
	return false
}

// onCombatFlowCompleteReq 确认客户端已完成全部战斗表现, 并在角色不处于战斗时恢复自动遇敌计时.
func (p *Account) onCombatFlowCompleteReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CombatFlowCompleteReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatFlowCompleteRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	if characterUUID == 0 {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatFlowCompleteRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.characterManager.find(characterUUID)
	if character == nil || character.record == nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatFlowCompleteRes_CMD), xerror.NotFound.Code())
		return
	}
	if !character.online || character.combatRoom != nil {
		p.sendClientErr(gateway, uint32(pb.MsgID_CombatFlowCompleteRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if character.autoEncounterEnabled && character.autoEncounterTimer == nil {
		character.restartAutoEncounterTimer(gateway)
	}
	p.sendClientRes(gateway, uint32(pb.MsgID_CombatFlowCompleteRes_CMD), xerror.Success.Code(), &pb.CombatFlowCompleteRes{
		CharacterUuid: characterUUID,
	})
}

// arrangeCombatPVEEnemyUnits复刻8.5 BATTLE_CreateVsEnemy创建完成后的敌方前后排交换.
//
// 调用者必须按敌人的原始创建顺序传入1至10个非nil单位. 8.5先让BATTLE_NewEntry
// 把敌人依次放入防守侧Entry[0]至Entry[9], 等玩家和默认宠物入场后再逐组交换
// Entry[0]<->Entry[5]、Entry[1]<->Entry[6]直到Entry[4]<->Entry[9].
// 因而创建序号i的最终阵营内位置固定为(i+5)%10: 前五只敌人移动到5至9,
// 第六至第十只移动到0至4.
//
// CombatUnitKey表示本场单位身份, 必须继续保留创建时分配的PetUuid, 不能按照
// 交换后的position重新编号. 完成位置改写后再按position升序排列紧凑切片,
// 等价于8.5后续按BattleArray.Entry[0..9]扫描. 该顺序会继续传给开战快照、
// 敌方AI、失效目标改选和结算单位列表, 防止这些流程各自得到不同的单位次序.
func arrangeCombatPVEEnemyUnits(enemyUnits []*pb.CombatUnit) {
	positionCount := int(pb.CombatCampPosition_CombatCampPosition_Count)
	for creationIndex, unit := range enemyUnits {
		unit.Position = uint32((creationIndex + combatCampRowPositionCount) % positionCount)
	}
	sort.SliceStable(enemyUnits, func(left int, right int) bool {
		return enemyUnits[left].GetPosition() < enemyUnits[right].GetPosition()
	})
}

// combatPVERandomRange统一描述敌人生成阶段使用的闭区间RAND.
//
// 生产环境传入xutil.RandomU32. 测试通过固定返回序列同时断言每次调用的上下界,
// 从而锁定8.5在“目标数量、候选权重、逐敌人等级”三个阶段的抽数顺序.
type combatPVERandomRange func(min uint32, max uint32) uint32

// selectCombatPVESceneEnemyGroup复刻ENEMY_getEnemy选择遇敌组的累计权重算法.
//
// 8.5先把当前遇敌区域内所有可用组的权重相加, 再执行一次RAND(0,total-1).
// 权重为0的组仍可保留在配置中, 但没有自己的随机区间, 因而不会被抽中.
// 道具出现条件、禁止出现条件和事件敌组覆盖依赖尚未接入的地图/NPC状态,
// 不在该纯权重函数中伪造, 继续由后续单角色PVE地图条件工作包实现.
func selectCombatPVESceneEnemyGroup(
	scene *gameconfig.SceneEntry,
	random combatPVERandomRange,
) (gameconfig.SceneEnemyGroupEntry, error) {
	if scene == nil || scene.ID == nil {
		return gameconfig.SceneEnemyGroupEntry{}, fmt.Errorf("scene or scene id is nil")
	}
	if random == nil {
		return gameconfig.SceneEnemyGroupEntry{}, fmt.Errorf("combat PVE random source is nil")
	}

	totalWeight := uint64(0)
	for index, group := range scene.EnemyGroups {
		if group.ID == nil || group.Weight == nil {
			return gameconfig.SceneEnemyGroupEntry{}, fmt.Errorf(
				"scene enemy group is incomplete: scene:%d index:%d", *scene.ID, index,
			)
		}
		totalWeight += uint64(*group.Weight)
	}
	if totalWeight == 0 || totalWeight > uint64(math.MaxInt32) {
		return gameconfig.SceneEnemyGroupEntry{}, fmt.Errorf(
			"scene enemy group total weight is invalid: scene:%d total:%d", *scene.ID, totalWeight,
		)
	}

	roll := random(0, uint32(totalWeight)-1)
	currentWeight := uint64(0)
	for _, group := range scene.EnemyGroups {
		currentWeight += uint64(*group.Weight)
		if uint64(roll) < currentWeight {
			return group, nil
		}
	}
	return gameconfig.SceneEnemyGroupEntry{}, fmt.Errorf(
		"scene enemy group weighted selection failed: scene:%d roll:%d total:%d",
		*scene.ID, roll, totalWeight,
	)
}

// selectCombatPVEEnemyEntries按enemy.group.yaml契约生成遇敌列表.
//
// 普通组先按配置顺序放入weight=0的必出敌人, 再从正权重池中有放回抽取:
//  1. 从countRange闭区间抽取本场敌人数.
//  2. 按配置顺序加入全部必出敌人.
//  3. 按累计正权重抽取其余敌人, 直到达到目标数量.
//
// Boss组不执行数量或权重随机, 直接保持配置顺序.
// 返回顺序随后交给arrangeCombatPVEEnemyUnits执行敌方阵营位置转换.
//
// 配置选择和位置转换是两个独立阶段.
func selectCombatPVEEnemyEntries(
	group *gameconfig.EnemyGroupEntry,
	random combatPVERandomRange,
) ([]gameconfig.EnemyEntry, error) {
	if group == nil || group.ID == nil || group.IsBoss == nil {
		return nil, fmt.Errorf("enemy group or required field is nil")
	}
	if *group.IsBoss {
		return append([]gameconfig.EnemyEntry(nil), group.Enemies...), nil
	}
	if random == nil {
		return nil, fmt.Errorf("combat PVE random source is nil")
	}
	if group.CountRange == nil || group.CountRange.Min == nil || group.CountRange.Max == nil {
		return nil, fmt.Errorf("normal enemy group count range is incomplete: group:%d", *group.ID)
	}

	targetCount := int(random(uint32(*group.CountRange.Min), uint32(*group.CountRange.Max)))
	selected := make([]gameconfig.EnemyEntry, 0, targetCount)
	totalWeight := uint64(0)
	for index, enemy := range group.Enemies {
		if enemy.ID == nil || enemy.Weight == nil {
			return nil, fmt.Errorf("normal enemy group entry is incomplete: group:%d index:%d", *group.ID, index)
		}
		if *enemy.Weight == 0 {
			selected = append(selected, enemy)
			continue
		}
		totalWeight += uint64(*enemy.Weight)
	}
	if len(selected) > targetCount {
		return nil, fmt.Errorf("required enemy count exceeds target: group:%d required:%d target:%d",
			*group.ID, len(selected), targetCount)
	}
	if len(selected) < targetCount && (totalWeight == 0 || totalWeight > uint64(math.MaxInt32)) {
		return nil, fmt.Errorf("enemy group total weight is invalid: group:%d total:%d", *group.ID, totalWeight)
	}

	for len(selected) < targetCount {
		roll := random(0, uint32(totalWeight)-1)
		selectedIndex := -1
		currentWeight := uint64(0)
		for index, enemy := range group.Enemies {
			currentWeight += uint64(*enemy.Weight)
			if uint64(roll) < currentWeight {
				selectedIndex = index
				break
			}
		}
		if selectedIndex < 0 {
			return nil, fmt.Errorf(
				"enemy weighted selection failed: group:%d roll:%d total:%d",
				*group.ID, roll, totalWeight,
			)
		}
		selected = append(selected, group.Enemies[selectedIndex])
	}

	return selected, nil
}

// combatPVEEnemyLevel返回本次创建敌人的等级.
//
// 条目显式level优先; 普通组否则按levelRange或玩家等级加roleLevelOffset抽取.
// 最终范围限制在协议等级[1,140]内.
func combatPVEEnemyLevel(
	group *gameconfig.EnemyGroupEntry,
	enemy gameconfig.EnemyEntry,
	roleLevel uint32,
	random combatPVERandomRange,
) (uint32, error) {
	if group == nil || group.ID == nil || group.IsBoss == nil || enemy.ID == nil {
		return 0, fmt.Errorf("enemy group or enemy level field is incomplete")
	}
	if enemy.Level != nil {
		return *enemy.Level, nil
	}
	if random == nil {
		return 0, fmt.Errorf("combat PVE random source is nil")
	}
	levelMin := int(pb.LevelRange_LevelRange_Min)
	levelMax := int(pb.LevelRange_LevelRange_Max)
	if group.LevelRange != nil && group.LevelRange.Min != nil && group.LevelRange.Max != nil {
		levelMin = *group.LevelRange.Min
		levelMax = *group.LevelRange.Max
	} else if group.RoleLevelOffset != nil &&
		group.RoleLevelOffset.Min != nil && group.RoleLevelOffset.Max != nil {
		levelMin = int(roleLevel) + *group.RoleLevelOffset.Min
		levelMax = int(roleLevel) + *group.RoleLevelOffset.Max
	} else {
		return 0, fmt.Errorf("enemy level range is invalid: group:%d enemy:%d", *group.ID, *enemy.ID)
	}
	if levelMin < int(pb.LevelRange_LevelRange_Min) {
		levelMin = int(pb.LevelRange_LevelRange_Min)
	}
	if levelMin > int(pb.LevelRange_LevelRange_Max) {
		levelMin = int(pb.LevelRange_LevelRange_Max)
	}
	if levelMax < int(pb.LevelRange_LevelRange_Min) {
		levelMax = int(pb.LevelRange_LevelRange_Min)
	}
	if levelMax > int(pb.LevelRange_LevelRange_Max) {
		levelMax = int(pb.LevelRange_LevelRange_Max)
	}
	if levelMax < levelMin {
		levelMax = levelMin
	}
	return random(uint32(levelMin), uint32(levelMax)), nil
}

// combatPVEEnemyAttributes保存一只敌人在本场战斗内的原始四维和WORK/FIX结果.
//
// 8.5的敌人不是账号中的持久宠物, 不保存PetRecord, 也不执行宠物按Rank逐级升级.
// 这组数据只在ENEMY_createEnemy等价流程中生成一次, 随后冻结到CombatUnit和
// combatUnitRuntimeState, 保证同一场战斗中的HP、攻、防、敏及状态公式共用同一份
// 随机个体属性.
type combatPVEEnemyAttributes struct {
	rawVitality  uint32
	rawStrength  uint32
	rawToughness uint32
	rawDexterity uint32
	hp           uint32
	attack       uint32
	defense      uint32
	agility      uint32
}

// createCombatPVEEnemyAttributes复刻8.5 ENEMY_createEnemy的无装备敌人属性生成链.
//
// 随机顺序和整数层级都属于战斗规则, 不能合并或交换:
//  1. 按体力、腕力、耐力、速度顺序各执行一次RAND(0,4), 映射为-2至+2.
//  2. 在四项已经偏移的基础值上执行10次RAND(0,3), 每次为命中的一项加1.
//  3. 旧服用atoi读取enemybase的LVUPPOINT, 因而文本4.50实际取整数4.
//  4. 使用(initNum+(level-1)*lvupPointEffective)*最终基础值一次性计算raw四维.
//  5. 按CHAR_initcharWorkInt的表达式顺序向零截断, 生成最大HP、攻击、防御和敏捷.
//
// 当前敌人组和宠物模板没有敌人装备或style字段, 因此本函数只生成无装备属性.
// 旧服style武器和敌人掉落装备需要正式配置模型及ITEM_equipEffect管线;
// 在数据归属明确前, 不在这里用临时倍率伪造装备效果.
func createCombatPVEEnemyAttributes(
	enemyPet *gameconfig.PetEntry,
	level uint32,
	random combatPVERandomRange,
) (combatPVEEnemyAttributes, error) {
	var attributes combatPVEEnemyAttributes
	if enemyPet == nil || enemyPet.ID == nil || enemyPet.Growth == nil {
		return attributes, fmt.Errorf("enemy pet or growth is nil")
	}
	if level < uint32(pb.LevelRange_LevelRange_Min) || level > uint32(pb.LevelRange_LevelRange_Max) {
		return attributes, fmt.Errorf("enemy level is out of range: pet:%d level:%d", *enemyPet.ID, level)
	}
	if random == nil {
		return attributes, fmt.Errorf("combat PVE random source is nil")
	}

	growth := enemyPet.Growth
	if growth.InitNum == nil ||
		growth.LvupPointSource == nil ||
		growth.BaseVital == nil ||
		growth.BaseStr == nil ||
		growth.BaseTough == nil ||
		growth.BaseDex == nil {
		return attributes, fmt.Errorf("enemy growth field is incomplete: pet:%d", *enemyPet.ID)
	}
	if *growth.InitNum == 0 || *growth.InitNum > uint32(math.MaxInt32) {
		return attributes, fmt.Errorf("enemy init number is outside C int range: pet:%d value:%d",
			*enemyPet.ID, *growth.InitNum)
	}
	lvupPointSource := *growth.LvupPointSource
	if math.IsNaN(lvupPointSource) ||
		math.IsInf(lvupPointSource, 0) ||
		lvupPointSource <= 0 ||
		lvupPointSource > float64(math.MaxInt32) {
		return attributes, fmt.Errorf("enemy level-up point is outside C int range: pet:%d value:%v",
			*enemyPet.ID, lvupPointSource)
	}

	// pet.yaml保留原文本的数值语义. 对正数直接转为uint64等价于旧服atoi在小数点
	// 处停止解析, 例如4.50得到4. 不能把该字段四舍五入或直接按4.5参与计算.
	lvupPointEffective := uint64(lvupPointSource)
	factor := uint64(*growth.InitNum) + uint64(level-1)*lvupPointEffective
	if factor == 0 || factor > uint64(math.MaxInt32) {
		return attributes, fmt.Errorf("enemy growth factor is outside C int range: pet:%d level:%d factor:%d",
			*enemyPet.ID, level, factor)
	}

	baseValues := [4]int64{
		int64(*growth.BaseVital),
		int64(*growth.BaseStr),
		int64(*growth.BaseTough),
		int64(*growth.BaseDex),
	}
	for index := range baseValues {
		// RAND(0,4)-2必须分别调用四次. 合并成一次品阶偏移会让四维完全相关,
		// 既改变个体分布, 也会破坏后续共享随机流的顺序.
		baseValues[index] += int64(random(0, 4)) - 2
		if baseValues[index] <= 0 || baseValues[index] > int64(math.MaxInt32) {
			return attributes, fmt.Errorf(
				"enemy randomized base attribute is outside C int range: pet:%d index:%d value:%d",
				*enemyPet.ID, index, baseValues[index],
			)
		}
	}
	for point := 0; point < combatPVEEnemyInitialBonusPointCount; point++ {
		index := random(0, 3)
		if index > 3 {
			return attributes, fmt.Errorf("enemy bonus attribute random result is invalid: pet:%d value:%d",
				*enemyPet.ID, index)
		}
		baseValues[index]++
		if baseValues[index] > int64(math.MaxInt32) {
			return attributes, fmt.Errorf(
				"enemy bonus base attribute is outside C int range: pet:%d index:%d value:%d",
				*enemyPet.ID, index, baseValues[index],
			)
		}
	}

	rawValues := [4]uint32{}
	for index, baseValue := range baseValues {
		rawValue := factor * uint64(baseValue)
		if rawValue > uint64(math.MaxInt32) {
			return attributes, fmt.Errorf(
				"enemy raw attribute is outside C int range: pet:%d index:%d value:%d",
				*enemyPet.ID, index, rawValue,
			)
		}
		rawValues[index] = uint32(rawValue)
	}
	attributes.rawVitality = rawValues[0]
	attributes.rawStrength = rawValues[1]
	attributes.rawToughness = rawValues[2]
	attributes.rawDexterity = rawValues[3]

	// 旧服先在C int中完成rawVital*4及四项求和, 再乘double 0.01并赋给
	// float hp. 这里先检查32位有符号整数范围, 再显式经过float32舍入后截断.
	hpInput := uint64(attributes.rawVitality)*4 +
		uint64(attributes.rawStrength) +
		uint64(attributes.rawToughness) +
		uint64(attributes.rawDexterity)
	if hpInput > uint64(math.MaxInt32) {
		return combatPVEEnemyAttributes{}, fmt.Errorf(
			"enemy max HP input is outside C int range: pet:%d value:%d", *enemyPet.ID, hpInput,
		)
	}
	attributes.hp = uint32(float32(float64(hpInput) * 0.01))

	// FIXSTR/FIXTOUGH在旧服以double表达式计算后直接传入int参数. 保留各乘法
	// 的原始顺序, 避免先合并常量或改用float32造成整数边界相差1.
	attributes.attack = uint32(
		float64(attributes.rawStrength)*0.01*1.0 +
			float64(attributes.rawToughness)*0.01*0.1 +
			float64(attributes.rawVitality)*0.01*0.1 +
			float64(attributes.rawDexterity)*0.01*0.05,
	)
	attributes.defense = uint32(
		float64(attributes.rawToughness)*0.01*1.0 +
			float64(attributes.rawStrength)*0.01*0.1 +
			float64(attributes.rawVitality)*0.01*0.1 +
			float64(attributes.rawDexterity)*0.01*0.05,
	)
	attributes.agility = uint32(float64(attributes.rawDexterity) * 0.01)
	if attributes.hp == 0 {
		return combatPVEEnemyAttributes{}, fmt.Errorf("enemy max HP is zero: pet:%d level:%d",
			*enemyPet.ID, level)
	}
	return attributes, nil
}

// combatPVEEnemyExperience根据宠物模板和等级生成敌人经验.
//
// enemy.group.yaml直接引用pet.yaml, 因此敌人经验只依赖共享宠物模板和
// enemy.exp.yaml, 不再经过独立敌人记录.
func combatPVEEnemyExperience(petID uint32, level uint32) (uint32, error) {
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.EnemyExp == nil {
		return 0, fmt.Errorf("enemy experience config is not loaded")
	}
	experience, err := gameconfig.GGameConfig.EnemyExp.GenerateEnemyExp(petID, level)
	if err != nil {
		return 0, fmt.Errorf("generate enemy experience failed pet:%d level:%d: %w",
			petID, level, err)
	}
	return experience, nil
}

// restartAutoEncounterTimer 重建自动遇敌定时器, 并在回调中创建完整 CombatRoom 后启动房间 actor.
func (c *character) restartAutoEncounterTimer(gateway *Gateway) {
	p := c.account
	// 始终先撤销旧任务, 保证同一角色最多只有一个有效的自动遇敌回调.
	c.clearAutoEncounterTimer()
	if xtimer.GTimer == nil {
		return
	}
	characterUUID := c.record.GetBase().GetUuid()
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if p.characterManager.find(characterUUID) != c {
			return nil
		}
		c.autoEncounterTimer = nil
		if !c.online || !c.autoEncounterEnabled || c.combatRoom != nil {
			return nil
		}
		// 使用统一错误出口处理建房失败, 确保任一步失败都不会发布半初始化的 CombatRoom.
		var encounterErr error
	startBattle:
		for {
			if gateway == nil {
				encounterErr = fmt.Errorf("gateway is nil")
				break
			}
			character, battlePet, err := c.currentBattleCharacterAndPet()
			if err != nil {
				encounterErr = err
				break
			}
			sceneEntry := p.getCharacterScene(character)
			if nil == sceneEntry {
				encounterErr = fmt.Errorf("scene entry is nil")
				break
			}
			if gameconfig.GGameConfig == nil ||
				gameconfig.GGameConfig.Scene == nil ||
				gameconfig.GGameConfig.Enemy == nil ||
				gameconfig.GGameConfig.Pet == nil {
				encounterErr = fmt.Errorf("scene, enemy group or pet config is not loaded")
				break
			}
			scene := gameconfig.GGameConfig.Scene.Get(*sceneEntry.ID)
			if scene == nil {
				encounterErr = fmt.Errorf("scene not found: %d", *sceneEntry.ID)
				break
			}
			selectedGroup, err := selectCombatPVESceneEnemyGroup(scene, xutil.RandomU32)
			if err != nil {
				encounterErr = err
				break
			}
			enemyGroup := gameconfig.GGameConfig.Enemy.Get(uint32(*selectedGroup.ID))
			if enemyGroup == nil {
				encounterErr = fmt.Errorf("scene enemy group not found: scene:%d group:%d", *sceneEntry.ID, *selectedGroup.ID)
				break
			}

			// 使用 xlib 生成密码学安全的 UUID 字符串, 作为客户端不解析的战斗唯一标识.
			battleID := xutil.UUIDRandomString()
			battleStart := &pb.CombatBattleStartNotify{BattleId: battleID}
			// 敌方AI目标选择需要CombatUnit协议未携带的原始腕力和速度. 这里按
			// 稳定单位键暂存开战快照, 等全部单位完成校验后与unitStates一起发布.
			targetAttributes := make(map[string]combatTargetAttributeSnapshot)
			if character == nil || character.GetBase().GetUuid() == 0 || character.GetBase().GetAssetId() == 0 {
				encounterErr = fmt.Errorf("character record invalid")
				break
			}
			// 角色属性在开战时固化为战斗快照. 四维全为 0 视为角色数据尚未正确初始化.
			vitality := uint64(character.GetBase().GetVitality())
			strength := uint64(character.GetBase().GetStrength())
			toughness := uint64(character.GetBase().GetToughness())
			dexterity := uint64(character.GetBase().GetDexterity())
			if vitality+strength+toughness+dexterity == 0 {
				encounterErr = fmt.Errorf("character attribute missing character:%d", character.GetBase().GetUuid())
				break
			}
			if gameconfig.GGameConfig.Exp == nil {
				encounterErr = fmt.Errorf("exp config is not loaded")
				break
			}
			characterLevel, err := gameconfig.GGameConfig.Exp.GetLevel(character.GetBase().GetExp())
			if err != nil {
				encounterErr = err
				break
			}
			// 当前角色档案尚未接入已装备物品集合, 因此装备修正列表为空.
			// 后续装备系统接入时, 每次从当前已装备物品配置解析修正值并传入聚合函数, 不持久化装备运气.
			luckSnapshot, err := newCombatLuckSnapshot(character.GetBase().GetLuckState().GetBaseLuck(), nil)
			if err != nil {
				encounterErr = fmt.Errorf("character luck invalid character:%d: %w", character.GetBase().GetUuid(), err)
				break
			}
			characterUnit := &pb.CombatUnit{
				Camp:        pb.CombatCamp_CombatCamp_Initiator,
				Position:    initiatorCharacterPosition,
				Key:         &pb.CombatUnitKey{Aid: p.aid, CharacterUuid: character.GetBase().GetUuid()},
				CharacterId: uint32(character.GetBase().GetAssetId()),
				Attribute: &pb.CombatUnitAttribute{
					Hp:      uint32(vitality*4 + strength + toughness + dexterity),
					Attack:  uint32(maxUint64(1, strength+toughness/10+vitality/10+dexterity/20)),
					Defense: uint32(maxUint64(1, toughness+strength/10+vitality/10+dexterity/20)),
					Agility: uint32(maxUint64(1, dexterity)),
					MaxMp:   uint32(pb.CharacterLimit_CharacterLimit_MagicPointMax),
					Elemental: &pb.ElementalPoints{
						Earth: character.GetBase().GetEarth(),
						Water: character.GetBase().GetWater(),
						Fire:  character.GetBase().GetFire(),
						Wind:  character.GetBase().GetWind(),
					},
					Level: characterLevel,
					Luck:  luckSnapshot,
				},
			}
			targetAttributes[combatUnitKeyMapKey(characterUnit.GetKey())] = combatTargetAttributeSnapshot{
				strength:  strength,
				dexterity: dexterity,
			}
			var petUnit *pb.CombatUnit
			if battlePet != nil {
				if battlePet.GetUuid() == 0 {
					encounterErr = fmt.Errorf("pet record invalid")
					break
				}
				petAssetID := battlePet.GetAssetId()
				if petAssetID == 0 {
					encounterErr = fmt.Errorf("pet asset id is invalid pet:%d asset:%d", battlePet.GetUuid(), petAssetID)
					break
				}
				petEntry := gameconfig.GGameConfig.Pet.Get(petAssetID)
				if petEntry == nil {
					encounterErr = fmt.Errorf("pet config not found: %d", petAssetID)
					break
				}
				petLevel, err := gameconfig.GGameConfig.Exp.GetLevel(battlePet.GetExp())
				if err != nil {
					encounterErr = err
					break
				}
				// 玩家宠物使用持久化的运行期原始四维和公共宠物公式生成战斗属性.
				// 敌人使用独立的8.5 ENEMY_createEnemy生成链, 不复用玩家宠物的Rank逐级成长.
				rawVitality := battlePet.GetRawVitality()
				rawStrength := battlePet.GetRawStrength()
				rawToughness := battlePet.GetRawToughness()
				rawDexterity := battlePet.GetRawDexterity()
				if rawVitality == 0 && rawStrength == 0 && rawToughness == 0 && rawDexterity == 0 {
					encounterErr = fmt.Errorf("pet runtime attribute missing pet:%d", battlePet.GetUuid())
					break
				}
				petUnit = &pb.CombatUnit{
					Camp:        pb.CombatCamp_CombatCamp_Initiator,
					Position:    initiatorPetPosition,
					Key:         &pb.CombatUnitKey{Aid: p.aid, CharacterUuid: character.GetBase().GetUuid(), PetUuid: battlePet.GetUuid()},
					CharacterId: uint32(character.GetBase().GetAssetId()),
					PetId:       petAssetID,
					Attribute: &pb.CombatUnitAttribute{
						Hp:        commonpet.CalculateHP(rawVitality, rawStrength, rawToughness, rawDexterity),
						Attack:    commonpet.CalculateAttack(rawVitality, rawStrength, rawToughness, rawDexterity),
						Defense:   commonpet.CalculateDefense(rawVitality, rawStrength, rawToughness, rawDexterity),
						Agility:   commonpet.CalculateAgility(rawDexterity),
						Loyalty:   battlePet.GetLoyalty(),
						Elemental: combatPetElementalPoints(petEntry),
						Level:     petLevel,
					},
				}
				targetAttributes[combatUnitKeyMapKey(petUnit.GetKey())] = combatTargetAttributeSnapshot{
					strength:  uint64(rawStrength),
					dexterity: uint64(rawDexterity),
				}
			}

			if len(enemyGroup.Enemies) == 0 {
				encounterErr = fmt.Errorf("enemy group is empty")
				break
			}
			selectedEnemies, err := selectCombatPVEEnemyEntries(enemyGroup, xutil.RandomU32)
			if err != nil {
				encounterErr = err
				break
			}

			enemyUnits := make([]*pb.CombatUnit, 0, len(selectedEnemies))
			pveEnemyKeys := make(map[string]struct{}, len(selectedEnemies))
			enemyExperiences := make(map[string]uint32, len(selectedEnemies))
			for index, enemy := range selectedEnemies {
				if enemy.ID == nil {
					encounterErr = fmt.Errorf("selected enemy pet id is nil")
					break startBattle
				}
				petID := *enemy.ID
				level, err := combatPVEEnemyLevel(enemyGroup, enemy, characterLevel, xutil.RandomU32)
				if err != nil {
					encounterErr = err
					break startBattle
				}
				enemyPet := gameconfig.GGameConfig.Pet.Get(petID)
				if enemyPet == nil {
					encounterErr = fmt.Errorf("enemy pet config not found: pet:%d", petID)
					break startBattle
				}
				enemyAttributes, err := createCombatPVEEnemyAttributes(enemyPet, level, xutil.RandomU32)
				if err != nil {
					encounterErr = err
					break startBattle
				}
				enemyExperience, err := combatPVEEnemyExperience(petID, level)
				if err != nil {
					encounterErr = err
					break startBattle
				}
				// 敌方没有账号内的持久化宠物 UUID, 使用从 1 开始的本场序号生成战斗内唯一的单位键.
				enemyUnit := &pb.CombatUnit{
					Camp:     pb.CombatCamp_CombatCamp_Defender,
					Position: uint32(index),
					Key:      &pb.CombatUnitKey{PetUuid: uint64(index) + 1},
					PetId:    petID,
					Attribute: &pb.CombatUnitAttribute{
						Hp:        enemyAttributes.hp,
						Attack:    enemyAttributes.attack,
						Defense:   enemyAttributes.defense,
						Agility:   enemyAttributes.agility,
						Elemental: combatPetElementalPoints(enemyPet),
						Level:     level,
					},
				}
				enemyUnits = append(enemyUnits, enemyUnit)
				enemyKey := combatUnitKeyMapKey(enemyUnit.GetKey())
				pveEnemyKeys[enemyKey] = struct{}{}
				enemyExperiences[enemyKey] = enemyExperience
				targetAttributes[enemyKey] = combatTargetAttributeSnapshot{
					strength:  uint64(enemyAttributes.rawStrength),
					dexterity: uint64(enemyAttributes.rawDexterity),
				}
			}
			// 8.5在全部敌人和玩家侧单位完成入场后交换敌方前后五格.
			// 这里保留创建时的临时单位键, 只改写最终position并按旧Entry扫描顺序排列.
			arrangeCombatPVEEnemyUnits(enemyUnits)
			battleStart.UnitList = append(battleStart.UnitList, characterUnit)
			if petUnit != nil {
				battleStart.UnitList = append(battleStart.UnitList, petUnit)
			}
			battleStart.UnitList = append(battleStart.UnitList, enemyUnits...)
			// 从不可变开战快照派生可变状态. 当前MP按开战上限满值初始化;
			// 后续扣血和扣MP只更新unitStates, 不污染已经发送给客户端的
			// battleStart开战通知.
			unitStates := make(map[string]*combatUnitRuntimeState, len(battleStart.GetUnitList()))
			for _, unit := range battleStart.GetUnitList() {
				if unit == nil || unit.GetKey() == nil {
					continue
				}
				maxHP := uint64(unit.GetAttribute().GetHp())
				maxMP := uint64(unit.GetAttribute().GetMaxMp())
				state := &combatUnitRuntimeState{
					unit:  unit,
					hp:    maxHP,
					maxHP: maxHP,
					mp:    maxMP,
					maxMP: maxMP,
					alive: maxHP > 0,
				}
				targetAttribute := targetAttributes[combatUnitKeyMapKey(unit.GetKey())]
				state.rawStrength = targetAttribute.strength
				state.rawDexterity = targetAttribute.dexterity
				unitKey := combatUnitKeyMapKey(unit.GetKey())
				_, state.pveEnemy = pveEnemyKeys[unitKey]
				state.enemyExperience = enemyExperiences[unitKey]
				if combatUnitKeyEqual(unit.GetKey(), characterUnit.GetKey()) {
					state.characterDuelPoint = character.GetBase().GetDuelPoint()
				}
				if unit.GetPetId() != 0 {
					applyPetBattleTraits(state, gameconfig.GGameConfig.Pet.Get(unit.GetPetId()))
				}
				unitStates[combatUnitKeyMapKey(unit.GetKey())] = state
			}

			// 所有配置和记录均验证成功后才发布 CombatRoom, 防止其他请求观察到半初始化房间.
			participant := &combatRoomParticipant{
				key: combatRoomParticipantKey{
					aid:           p.aid,
					characterUUID: characterUUID,
				},
				account:         p,
				gateway:         gateway,
				playerCharacter: characterUnit,
				playerPet:       petUnit,
			}
			room, err := newCombatRoom(battleID, participant, battleStart, enemyUnits, unitStates)
			if err != nil {
				encounterErr = err
				break
			}
			c.combatRoom = room
			room.PostStart()
			break
		}
		if encounterErr != nil {
			// 自动遇敌失败后关闭开关, 避免配置或角色状态持续异常时形成无限重试和日志风暴.
			xlog.GLog.Warnf("auto encounter failed aid:%d character:%d err:%v", p.aid, characterUUID, encounterErr)
			c.autoEncounterEnabled = false
			c.clearAutoEncounterTimer()
			c.notifyAutoEncounterState(gateway)
		}
		return nil
	})
	c.autoEncounterTimer = xtimer.GTimer.AddSecond(cb, time.Now().Add(autoEncounterInterval).Unix(), p.actor)
}

// clearAutoEncounterTimer 关闭当前自动遇敌任务; xlib 的开关会同时阻止定时器容器和 actor 队列中的回调执行.
func (c *character) clearAutoEncounterTimer() {
	if xtimer.GTimer != nil && c.autoEncounterTimer != nil {
		xtimer.GTimer.DelSecond(c.autoEncounterTimer)
	}
	c.autoEncounterTimer = nil
}

// notifyAutoEncounterState 复用设置回复协议主动同步目标角色的权威状态.
func (c *character) notifyAutoEncounterState(gateway *Gateway) {
	if c == nil || c.account == nil || c.record == nil {
		return
	}
	c.account.sendClientRes(gateway, uint32(pb.MsgID_CombatAutoEncounterSetRes_CMD), xerror.Success.Code(), &pb.CombatAutoEncounterSetRes{
		CharacterUuid:     c.record.GetBase().GetUuid(),
		Enabled:           c.autoEncounterEnabled,
		ServerTimestampMs: time.Now().UnixMilli(),
	})
}

// beginCombatRound重置房间回合状态并注册100秒超时处理.
func (r *CombatRoom) beginCombatRound() {
	if r == nil || r.roundTimer != nil || xtimer.GTimer == nil {
		return
	}
	r.playerActions = make(map[string]*combatAction)
	r.resetRoundGuards()
	battleID := r.battleID
	round := r.round
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if r.roundTimer == nil || r.battleID != battleID || r.round != round {
			return nil
		}
		// 服务端为缺失技能输入的存活玩家角色和宠物直接补对应防御技能, 不依赖宠物技能槽.
		r.completeCombatRound(r.playerActionsWithTimeoutDefaults())
		return nil
	})
	r.roundTimer = xtimer.GTimer.AddSecond(cb, time.Now().Add(combatRoundDuration).Unix(), r.actor)
}

func (r *CombatRoom) playerActionsWithTimeoutDefaults() []*combatAction {
	actions := make([]*combatAction, 0, len(r.participantOrder)*2)
	for _, participantKey := range r.participantOrder {
		participant := r.participant(participantKey)
		if participant == nil {
			continue
		}
		for _, unit := range []*pb.CombatUnit{participant.playerCharacter, participant.playerPet} {
			if unit == nil || !r.isAlive(unit.GetKey()) {
				continue
			}
			if action := r.playerActions[combatUnitKeyMapKey(unit.GetKey())]; action != nil {
				actions = append(actions, action)
				continue
			}
			actions = append(actions, defaultCombatAction(unit))
		}
	}
	return actions
}

func defaultCombatAction(unit *pb.CombatUnit) *combatAction {
	action := &combatAction{
		unitKey:   cloneCombatUnitKey(unit.GetKey()),
		kind:      combatActionKindDefense,
		targetKey: cloneCombatUnitKey(unit.GetKey()),
	}
	if combatUnitIsCharacter(unit) {
		action.skillID = combatSkillDefense
		return action
	}
	action.skillID = combatSkillDefense
	return action
}

// clearRoundTimer 关闭当前回合任务; xlib 的开关会同时阻止定时器容器和 actor 队列中的回调执行.
func (r *CombatRoom) clearRoundTimer() {
	if xtimer.GTimer != nil && r.roundTimer != nil {
		xtimer.GTimer.DelSecond(r.roundTimer)
	}
	r.roundTimer = nil
}

// playerActionsReady 判断所有仍存活的玩家可控单位是否都已提交动作.
func (r *CombatRoom) playerActionsReady() bool {
	if r == nil || r.roundTimer == nil {
		return false
	}
	for _, key := range r.requiredPlayerUnitKeys() {
		if _, ok := r.playerActions[combatUnitKeyMapKey(key)]; !ok {
			return false
		}
	}
	return true
}

// collectedPlayerActions 按玩家可控单位顺序收集当前回合已锁定的动作.
func (r *CombatRoom) collectedPlayerActions() []*combatAction {
	if r == nil {
		return nil
	}
	actions := make([]*combatAction, 0, len(r.participantOrder)*2)
	for _, key := range r.requiredPlayerUnitKeys() {
		if action := r.playerActions[combatUnitKeyMapKey(key)]; action != nil {
			actions = append(actions, action)
		}
	}
	return actions
}

// clearRuntime 取消当前角色的自动遇敌任务, 通知 CombatRoom 离开并清空房间引用.
func (c *character) clearRuntime() {
	if c == nil {
		return
	}
	c.clearAutoEncounterTimer()
	if c.combatRoom != nil && c.account != nil && c.record != nil {
		c.combatRoom.PostLeave(combatRoomParticipantKey{
			aid:           c.account.aid,
			characterUUID: c.record.GetBase().GetUuid(),
		})
	}
	c.combatRoom = nil
	c.autoEncounterEnabled = false
}

// currentBattleCharacterAndPet 返回当前角色及可选的唯一战斗状态宠物, 没有战斗宠物时返回 nil 且不报错.
func (c *character) currentBattleCharacterAndPet() (*pb.CharacterRecord, *pb.PetRecord, error) {
	p := c.account
	// 战斗只能基于已经完成初始化的账号快照, 避免用零值记录生成无法持久化追踪的单位.
	if p.accountRecord == nil {
		return nil, nil, fmt.Errorf("account record is not initialized")
	}
	// 角色单元必须在线且仍引用有效档案, 防止下线或重绑后的旧 timer 继续触发战斗.
	if !c.online || c.record == nil || c.record.GetBase().GetUuid() == 0 {
		return nil, nil, fmt.Errorf("online character not found")
	}
	character := c.record
	// 携带状态由角色记录提供; 当前约定每个角色至多有一只 Battle 状态宠物, 命中后直接返回.
	for _, pet := range character.GetPetRecordList() {
		if pet != nil && pet.GetCarryStatus() == pb.PetCarryStatus_PetCarryStatus_Battle {
			return character, pet, nil
		}
	}
	return character, nil, nil
}

// combatParticipantBattleReward保存一名参与者在正常PVE胜利时可以进入
// Account聚合根的最终战后奖励输入.
//
// 经验是逐敌人衰减后的整场累计值, 尚未经过角色/宠物最高经验上限;
// itemAssetIDs是战内getitem三格最终保留结果, 尚未经过现代背包容量判断.
// 各类最终实际写入值都由聚合根持久化函数返回.
type combatParticipantBattleReward struct {
	characterExperience     uint64
	duelPointBattle         bool
	characterDuelPointDelta int64
	battlePetUUID           uint64
	battlePetExperience     uint64
	itemAssetIDs            []uint32
}

// playerCombatBattleReward读取房间中一名参与者最终可结算的战后奖励.
//
// 只有接收角色所属阵营获胜时才进入EXP结算. 角色要求运行
// 态仍存在、存活且未逃离; 战宠只要运行态仍存在且存活, 即使已经回到宠物
// 栏也取得此前累计经验. 败方、主动逃跑角色、掉线和死亡单位不取得经验.
// 宠物经验写入宠物档案, 但随所属角色的最终结算发送.
func (r *CombatRoom) playerCombatBattleReward(
	key combatRoomParticipantKey,
	battleResult pb.CombatBattleResult,
) (combatParticipantBattleReward, error) {
	var reward combatParticipantBattleReward
	if r == nil {
		return reward, fmt.Errorf("combat room is nil")
	}
	var winnerCamp pb.CombatCamp
	switch battleResult {
	case pb.CombatBattleResult_CombatBattleResult_InitiatorWin:
		winnerCamp = pb.CombatCamp_CombatCamp_Initiator
	case pb.CombatBattleResult_CombatBattleResult_DefenderWin:
		winnerCamp = pb.CombatCamp_CombatCamp_Defender
	default:
		return reward, fmt.Errorf("invalid combat battle result: %s", battleResult.String())
	}
	participant := r.participant(key)
	if participant == nil || participant.playerCharacter == nil {
		return reward, fmt.Errorf("combat participant player character not found")
	}
	characterState := r.stateByKey(participant.playerCharacter.GetKey())
	if characterState == nil {
		return reward, nil
	}
	if r.pveDuelPointBattle {
		// DP战斗的BATTLE_GetProfit改调BATTLE_GetDuelPoint, 完全跳过
		// BATTLE_GetExpGold. 玩家角色即使死亡也要应用先前累计的负DP;
		// 主动逃离时Entry已在BATTLE_Finish前移除, 因而不结算任何DP.
		if characterState.escaped {
			return reward, nil
		}
		reward.duelPointBattle = true
		reward.characterDuelPointDelta = characterState.battleDuelPoint
		return reward, nil
	}
	if participant.playerCharacter.GetCamp() != winnerCamp {
		return reward, nil
	}
	if !characterState.alive || characterState.escaped {
		return reward, nil
	}
	reward.itemAssetIDs = append([]uint32(nil), characterState.battleDropAssetIDs...)
	reward.characterExperience = characterState.battleExperience
	if participant.playerPet == nil {
		return reward, nil
	}
	petState := r.stateByKey(participant.playerPet.GetKey())
	if petState == nil {
		return reward, nil
	}
	if !petState.alive {
		return reward, nil
	}
	reward.battlePetUUID = participant.playerPet.GetKey().GetPetUuid()
	reward.battlePetExperience = petState.battleExperience
	return reward, nil
}

// combatParticipantPersistenceInput保存一次战斗结束原子写入的全部服务器结果.
//
// 字段都由CombatRoom权威运行态产生, 不接受客户端经验、宠物UUID或状态.
// battlePetExperience为0时battlePetUUID可以为0; 非0经验必须明确指定本场战宠.
type combatParticipantPersistenceInput struct {
	characterExperience     uint64
	settleDuelPoint         bool
	characterDuelPointDelta int64
	battlePetUUID           uint64
	battlePetExperience     uint64
	itemAssetIDs            []uint32
}

// combatParticipantPersistenceResult返回实际发生的聚合根变化和上限结算结果.
type combatParticipantPersistenceResult struct {
	baseChanged               bool
	duelPointChanged          bool
	itemBagChanged            bool
	changedPet                *pb.PetRecord
	characterExperience       experienceSettlement
	battlePetExperience       experienceSettlement
	characterDuelPointDelta   int64
	characterDuelPointAfter   uint32
	receivedItemAssetIDs      []uint32
	discardedItemAssetIDs     []uint32
	receivedItemFinalCountMap map[uint32]uint64
	receivedEquipmentRecords  []*pb.EquipmentRecord
}

// combatCharacterDuelPointAfter复刻BATTLE_GetDuelPoint对CHAR_DUELPOINT的
// 有符号加减和0至CHAR_MAXDUELPOINT钳制.
//
// delta复用旧CHAR_WORKGETEXP槽, DP战中允许为负. 这里使用无溢出的分支
// 算法处理math.MinInt64, 避免先执行-delta造成二次溢出. 当前角色DP字段是
// uint32, 但业务上限仍严格使用8.5的一亿.
func combatCharacterDuelPointAfter(current uint32, delta int64) uint32 {
	const maximumDuelPoint = uint64(pb.CharacterLimit_CharacterLimit_MaxDuelPoint)
	currentValue := uint64(current)
	if delta >= 0 {
		added := uint64(delta)
		if added > maximumDuelPoint || currentValue > maximumDuelPoint-added {
			return uint32(maximumDuelPoint)
		}
		return uint32(currentValue + added)
	}
	subtracted := uint64(-(delta + 1)) + 1
	if subtracted >= currentValue {
		return 0
	}
	after := currentValue - subtracted
	if after > maximumDuelPoint {
		after = maximumDuelPoint
	}
	return uint32(after)
}

// combatDropPersistenceEntry保存一件已经通过配置、背包和UUID检查的掉落.
//
// equipmentUUID为0表示可堆叠普通道具; 非0表示需要创建该UUID的装备实例.
// 计划与应用分离, 保证所有本地输入和容量检查先于角色、账号状态修改.
type combatDropPersistenceEntry struct {
	assetID       uint32
	equipmentUUID uint64
}

type combatDropPersistencePlan struct {
	accepted      []combatDropPersistenceEntry
	discarded     []uint32
	nextUsedUUID  uint64
	usesAccountID bool
}

// planCombatDropPersistence把8.5临时getitem三格转换成当前背包可执行计划.
//
// 8.5每件物品都是独立ITEM实例, 当前项目则把普通道具按资产ID堆叠、装备按
// UUID保存实例. 因此容量判断遵循当前权威容器定义: 新普通道具种类和每件
// 装备各占一个条目, 已有普通道具继续堆叠不新增条目. 背包已满、普通堆叠
// 达到uint64上限或账号UUID耗尽时只丢弃当前物品, 与旧服结束时没有空物品格
// 就销毁战利品且不让整场战斗失败的语义一致.
func planCombatDropPersistence(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	assetIDs []uint32,
) (combatDropPersistencePlan, error) {
	var plan combatDropPersistencePlan
	if len(assetIDs) == 0 {
		if accountRecord != nil {
			plan.nextUsedUUID = accountRecord.GetUsedUuid()
		}
		return plan, nil
	}
	if characterRecord == nil || len(assetIDs) > combatPVEGetItemMax {
		return plan, fmt.Errorf("combat drop persistence argument invalid count:%d", len(assetIDs))
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Item == nil {
		return plan, fmt.Errorf("combat drop item config is not loaded")
	}

	const maximumBagCount = int(pb.CharacterLimit_CharacterLimit_MaxItemBagCount)
	container := characterRecord.GetItemBag()
	slotCount := itemContainerCount(container)
	simulatedItemCounts := make(map[uint32]uint64)
	if container != nil {
		for assetID, count := range container.GetItemCountMap() {
			simulatedItemCounts[assetID] = count
		}
	}
	if accountRecord != nil {
		plan.nextUsedUUID = accountRecord.GetUsedUuid()
	}

	for _, assetID := range assetIDs {
		if assetID == 0 || gameconfig.GGameConfig.Item.Get(assetID) == nil {
			return combatDropPersistencePlan{}, fmt.Errorf(
				"combat drop item config not found asset:%d",
				assetID,
			)
		}
		switch {
		case assetID >= uint32(pb.AssetIDRange_AssetIDRange_Item_Item_Start) &&
			assetID <= uint32(pb.AssetIDRange_AssetIDRange_Item_Item_End):
			current, exists := simulatedItemCounts[assetID]
			if current == math.MaxUint64 || (!exists && slotCount >= maximumBagCount) {
				plan.discarded = append(plan.discarded, assetID)
				continue
			}
			if !exists {
				slotCount++
			}
			simulatedItemCounts[assetID] = current + 1
			plan.accepted = append(plan.accepted, combatDropPersistenceEntry{assetID: assetID})
		case assetID >= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Start) &&
			assetID <= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_End):
			if slotCount >= maximumBagCount || accountRecord == nil ||
				plan.nextUsedUUID == math.MaxUint64 {
				plan.discarded = append(plan.discarded, assetID)
				continue
			}
			plan.nextUsedUUID++
			plan.usesAccountID = true
			slotCount++
			plan.accepted = append(plan.accepted, combatDropPersistenceEntry{
				assetID:       assetID,
				equipmentUUID: plan.nextUsedUUID,
			})
		default:
			return combatDropPersistencePlan{}, fmt.Errorf(
				"combat drop asset is outside item ranges asset:%d",
				assetID,
			)
		}
	}
	return plan, nil
}

// persistCombatParticipantResult原子持久化一名PVE参与者的战后服务器结果.
//
// 角色/战宠EXP、升级点、魅力、DP、宠物成长、普通道具、装备及账号
// used_uuid必须合并为一次cache写入, 防止只保存一部分. 函数先完成宠物
// 存在性、UUID唯一性和掉落入包计划, 再修改聚合根; 任一步计算或cache失败时
// 恢复全部字段并暴露错误. 返回结果没有任何变化时不调用persist.
func persistCombatParticipantResult(
	accountRecord *pb.AccountRecord,
	characterRecord *pb.CharacterRecord,
	input combatParticipantPersistenceInput,
	persist func() error,
) (combatParticipantPersistenceResult, error) {
	var result combatParticipantPersistenceResult
	if characterRecord == nil || characterRecord.GetBase() == nil || persist == nil {
		return result, fmt.Errorf("combat participant persistence argument invalid")
	}
	base := characterRecord.GetBase()
	if input.battlePetExperience > 0 && input.battlePetUUID == 0 {
		return result, fmt.Errorf("combat participant battle pet experience has no pet uuid character:%d", base.GetUuid())
	}
	dropPlan, err := planCombatDropPersistence(accountRecord, characterRecord, input.itemAssetIDs)
	if err != nil {
		return result, fmt.Errorf("combat participant drop plan failed character:%d: %w", base.GetUuid(), err)
	}
	result.discardedItemAssetIDs = append([]uint32(nil), dropPlan.discarded...)
	duelPointAfter := base.GetDuelPoint()
	if input.settleDuelPoint {
		duelPointAfter = combatCharacterDuelPointAfter(
			base.GetDuelPoint(),
			input.characterDuelPointDelta,
		)
	}

	var target *pb.PetRecord
	targetPetUUID := input.battlePetUUID
	if targetPetUUID != 0 {
		for _, petRecord := range characterRecord.GetPetRecordList() {
			if petRecord == nil || petRecord.GetUuid() != targetPetUUID {
				continue
			}
			if target != nil {
				return result, fmt.Errorf("combat battle pet uuid duplicated pet:%d", targetPetUUID)
			}
			target = petRecord
		}
		if target == nil {
			return result, fmt.Errorf("combat battle pet not found pet:%d", targetPetUUID)
		}
	}

	// EXP升级会同时改动多个角色字段和宠物成长字段, 掉落还会改动背包及账号
	// used_uuid. 逐字段手写回滚很容易在后续增加成长字段时遗漏, 因此对本函数
	// 实际可能修改的protobuf子对象建立深拷贝, 并单独保存直属标量. 这样既
	// 尽量保留聚合根内既有指针身份, 又能在计算或cache失败时完整恢复内容.
	previousBase := proto.Clone(base).(*pb.CharacterBaseRecord)
	var previousItemBag *pb.ItemContainerRecord
	if characterRecord.GetItemBag() != nil {
		previousItemBag = proto.Clone(characterRecord.GetItemBag()).(*pb.ItemContainerRecord)
	}
	var previousUsedUUID uint64
	if accountRecord != nil {
		previousUsedUUID = accountRecord.GetUsedUuid()
	}
	var previousPet *pb.PetRecord
	if target != nil {
		previousPet = proto.Clone(target).(*pb.PetRecord)
	}
	rollback := func() {
		proto.Reset(base)
		proto.Merge(base, previousBase)
		if previousItemBag == nil {
			characterRecord.ItemBag = nil
		} else {
			if characterRecord.ItemBag == nil {
				characterRecord.ItemBag = &pb.ItemContainerRecord{}
			}
			proto.Reset(characterRecord.ItemBag)
			proto.Merge(characterRecord.ItemBag, previousItemBag)
		}
		if accountRecord != nil {
			accountRecord.UsedUuid = previousUsedUUID
		}
		if target != nil {
			proto.Reset(target)
			proto.Merge(target, previousPet)
		}
	}

	base.DuelPoint = duelPointAfter
	if input.characterExperience > 0 {
		settlement, err := applyCharacterExperience(characterRecord, input.characterExperience)
		if err != nil {
			rollback()
			return combatParticipantPersistenceResult{}, fmt.Errorf(
				"apply combat character experience failed character:%d: %w",
				base.GetUuid(), err,
			)
		}
		result.characterExperience = settlement
	}
	if target != nil && input.battlePetExperience > 0 {
		settlement, err := applyPetExperience(target, input.battlePetExperience)
		if err != nil {
			rollback()
			return combatParticipantPersistenceResult{}, fmt.Errorf(
				"apply combat pet experience failed character:%d pet:%d: %w",
				base.GetUuid(), target.GetUuid(), err,
			)
		}
		result.battlePetExperience = settlement
	}
	if len(dropPlan.accepted) > 0 {
		if characterRecord.ItemBag == nil {
			characterRecord.ItemBag = &pb.ItemContainerRecord{}
		}
		if characterRecord.ItemBag.ItemCountMap == nil {
			characterRecord.ItemBag.ItemCountMap = make(map[uint32]uint64)
		}
		if characterRecord.ItemBag.EquipmentRecordMap == nil {
			characterRecord.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
		}
		result.receivedItemFinalCountMap = make(map[uint32]uint64)
		for _, entry := range dropPlan.accepted {
			result.receivedItemAssetIDs = append(result.receivedItemAssetIDs, entry.assetID)
			if entry.equipmentUUID == 0 {
				characterRecord.ItemBag.ItemCountMap[entry.assetID]++
				result.receivedItemFinalCountMap[entry.assetID] =
					characterRecord.ItemBag.ItemCountMap[entry.assetID]
				continue
			}
			equipment := &pb.EquipmentRecord{
				Uuid:    entry.equipmentUUID,
				AssetId: entry.assetID,
			}
			characterRecord.ItemBag.EquipmentRecordMap[entry.equipmentUUID] = equipment
			result.receivedEquipmentRecords = append(result.receivedEquipmentRecords, equipment)
		}
		if dropPlan.usesAccountID {
			accountRecord.UsedUuid = dropPlan.nextUsedUUID
		}
	}

	result.baseChanged = !proto.Equal(base, previousBase)
	result.itemBagChanged = !proto.Equal(characterRecord.GetItemBag(), previousItemBag)
	// 普通战斗升级也可能按照8.5成长表增加DP, 但那属于角色等级成长结果,
	// 不能伪装成“DP战斗直接结算”; 只有直接结算差量才写入结构化战斗结算.
	// 只有本场已经锁定为DP战斗, 且确实执行了直接DP结算时, 才设置该标志.
	result.duelPointChanged = input.settleDuelPoint &&
		base.GetDuelPoint() != previousBase.GetDuelPoint()
	result.characterDuelPointDelta = int64(base.GetDuelPoint()) - int64(previousBase.GetDuelPoint())
	result.characterDuelPointAfter = base.GetDuelPoint()
	if target != nil && !proto.Equal(target, previousPet) {
		result.changedPet = target
	}
	if !result.baseChanged && !result.itemBagChanged &&
		result.changedPet == nil {
		return result, nil
	}
	if err := persist(); err != nil {
		characterUUID := base.GetUuid()
		rollback()
		return combatParticipantPersistenceResult{}, fmt.Errorf(
			"persist combat participant result failed character:%d: %w",
			characterUUID, err,
		)
	}
	return result, nil
}

// resetRoundGuards 清除全部单位上一回合遗留的防御标记.
func (r *CombatRoom) resetRoundGuards() {
	if r == nil {
		return
	}
	for _, state := range r.unitStates {
		state.guard = false
	}
}

// requiredPlayerUnitKeys 返回当前回合仍存活且必须由玩家提交动作的单位键.
func (r *CombatRoom) requiredPlayerUnitKeys() []*pb.CombatUnitKey {
	if r == nil {
		return nil
	}
	// 死亡单位不再阻塞回合就绪; 返回顺序固定为参与者加入顺序下的角色, 宠物, 也决定动作收集顺序.
	keys := make([]*pb.CombatUnitKey, 0, len(r.participantOrder)*2)
	for _, participantKey := range r.participantOrder {
		participant := r.participants[participantKey]
		if participant == nil {
			continue
		}
		if participant.playerCharacter != nil && r.isAlive(participant.playerCharacter.GetKey()) {
			keys = append(keys, cloneCombatUnitKey(participant.playerCharacter.GetKey()))
		}
		if participant.playerPet != nil && r.isAlive(participant.playerPet.GetKey()) {
			keys = append(keys, cloneCombatUnitKey(participant.playerPet.GetKey()))
		}
	}
	return keys
}

// stateByKey 按稳定单位键查询战斗运行期状态, 无效键返回 nil.
func (r *CombatRoom) stateByKey(key *pb.CombatUnitKey) *combatUnitRuntimeState {
	if r == nil || key == nil {
		return nil
	}
	return r.unitStates[combatUnitKeyMapKey(key)]
}

// isAlive 判断指定单位是否有HP且仍在场内; 成功逃跑的单位不再参加目标、行动和胜负计算.
func (r *CombatRoom) isAlive(key *pb.CombatUnitKey) bool {
	state := r.stateByKey(key)
	return state != nil && state.alive && !state.escaped
}

// unitCamp 返回单位所属阵营, 第二个返回值表示单位是否有效.
func (r *CombatRoom) unitCamp(key *pb.CombatUnitKey) (pb.CombatCamp, bool) {
	state := r.stateByKey(key)
	if state == nil || state.unit == nil {
		return pb.CombatCamp_CombatCamp_Initiator, false
	}
	return state.unit.GetCamp(), true
}

// validOpponentTarget 校验请求目标存在、存活且与动作来源分属不同阵营.
func (r *CombatRoom) validOpponentTarget(requested *pb.CombatUnitKey, source *pb.CombatUnitKey) (*pb.CombatUnitKey, error) {
	// 依次验证目标是否提供, 目标是否存活, 来源是否存在以及阵营是否不同, 错误信息用于服务端诊断非法动作.
	if requested == nil || combatUnitKeyEmpty(requested) {
		return nil, fmt.Errorf("target is required")
	}
	if !r.isAlive(requested) {
		return nil, fmt.Errorf("target is not alive")
	}
	sourceCamp, ok := r.unitCamp(source)
	if !ok {
		return nil, fmt.Errorf("source unit not found")
	}
	targetCamp, ok := r.unitCamp(requested)
	if !ok {
		return nil, fmt.Errorf("target unit not found")
	}
	if sourceCamp == targetCamp {
		return nil, fmt.Errorf("target must be opponent")
	}
	// 返回副本而不是客户端请求中的 protobuf 指针, 避免意图对象引用外部可变消息.
	return cloneCombatUnitKey(requested), nil
}

// aliveOpponentKeys 按开战单位顺序返回来源单位的全部存活敌方目标.
func (r *CombatRoom) aliveOpponentKeys(source *pb.CombatUnitKey) []*pb.CombatUnitKey {
	if r == nil || source == nil {
		return nil
	}
	sourceCamp, ok := r.unitCamp(source)
	if !ok {
		return nil
	}
	// 遍历 battleStart 而不是 map, 从而保留开战时的稳定站位顺序.
	keys := make([]*pb.CombatUnitKey, 0)
	for _, unit := range r.battleStart.GetUnitList() {
		if unit.GetCamp() == sourceCamp || !r.isAlive(unit.GetKey()) {
			continue
		}
		keys = append(keys, cloneCombatUnitKey(unit.GetKey()))
	}
	return keys
}

// addPVEEnemyDefeatProfit复刻BATTLE_AddProfit的一次死亡扫描.
//
// 8.5不是等战斗胜利后把全部死敌平均分配给存活单位, 而是在每次动作、合击
// 或反击的实际攻击列表完成后, 扫描“HP<=0但尚未标记CHAR_ISDIE”的单位:
//   - 普通动作只把本动作来源放入攻击列表.
//   - 合击把经过状态筛选后真正进入BATTLE_Combo的全部成员放入列表.
//   - 每次反击单独用该次反击者作为列表.
//   - 敌方动作、反射、自身状态等也会完成一次死亡标记, 但敌方来源没有
//     玩家身份, 所以不会把经验错误发给下一名玩家行动者.
//
// 普通战调用BATTLE_AddExpItem等价分支, 只累计逐敌人等级差衰减后的EXP.
// 任一敌人DUELPOINT大于0时整场改调BATTLE_AddDuelPoint等价分支, EXP和
// 掉落全部停用, 并额外处理玩家死亡扣DP. DP钳制、最终等级提升和持久化
// 都必须等BATTLE_Finish等价结算点统一执行.
func (r *CombatRoom) addPVEEnemyDefeatProfit(attackerKeys []*pb.CombatUnitKey) {
	if r == nil || r.battleStart == nil || len(attackerKeys) == 0 {
		return
	}
	if r.pveDuelPointBattle {
		r.addPVEDuelPointDefeatProfit(attackerKeys)
		return
	}
	for _, unit := range r.battleStart.GetUnitList() {
		enemy := r.stateByKey(unit.GetKey())
		if enemy == nil || enemy.unit == nil || !enemy.pveEnemy ||
			enemy.alive || enemy.escaped || enemy.defeatProfitProcessed {
			continue
		}

		// 旧BATTLE_AddExpItem无论攻击列表来自玩家侧还是敌方侧, 都会在本次
		// 扫描末尾把敌人标成已经处理. 因此必须先锁定一次性标记, 不能仅在
		// 至少一名玩家取得经验时才标记.
		enemy.defeatProfitProcessed = true
		r.addPVEEnemyDropProfit(enemy, attackerKeys)
		seenAttackers := make(map[string]struct{}, len(attackerKeys))
		for _, attackerKey := range attackerKeys {
			attackerKeyString := combatUnitKeyMapKey(attackerKey)
			if _, exists := seenAttackers[attackerKeyString]; exists {
				continue
			}
			seenAttackers[attackerKeyString] = struct{}{}

			attacker := r.stateByKey(attackerKey)
			if attacker == nil || attacker.unit == nil || attacker.unit.GetAttribute() == nil ||
				attacker.unit.GetCamp() == enemy.unit.GetCamp() {
				continue
			}
			key := attacker.unit.GetKey()
			// 当前单角色PVE中, 玩家角色和其战宠都必须带aid+character_uuid;
			// 服务端生成的敌人key没有这两个字段. 该身份判断等价于旧服
			// proflg要求“攻击方Side是PLAYER且对方Side不是PLAYER”.
			if key.GetAid() == 0 || key.GetCharacterUuid() == 0 {
				continue
			}
			attacker.battleExperience += gameconfig.CalculateEnemyDefeatExperience(
				enemy.enemyExperience,
				attacker.unit.GetAttribute().GetLevel(),
				enemy.unit.GetAttribute().GetLevel(),
			)
		}
	}
}

// addPVEEnemyDropProfit复刻BATTLE_AddExpItem中先于EXP执行的战利品归属链.
//
// enemyDropAssetIDs已经在ENEMY_createEnemy等价阶段按十个槽位顺序抽出.
// 敌人本次首次死亡时, 每件物品继续执行以下随机和容量规则:
//  1. 从实际攻击列表中等概率选择一个Entry. 玩家战宠映射到主人角色,
//     但“角色+自己的战宠”仍保留为两个候选Entry并消费原有归属随机.
//  2. 主人临时getitem不足3件时写入第一个空槽.
//  3. 已满3件时执行RAND(0,1). 结果为0则删除新物品; 非0时再执行
//     RAND(0,2), 用新物品替换指定旧槽, 被替换物品立即删除.
//
// 当前运行态只保存最终3个现代资产ID, 不创建旧服ITEM实例. 这不改变上述
// 随机顺序和最终保留模板; 真正的现代道具或装备实例统一到战斗结束持久化.
func (r *CombatRoom) addPVEEnemyDropProfit(
	enemy *combatUnitRuntimeState,
	attackerKeys []*pb.CombatUnitKey,
) {
	if r == nil || r.random == nil || enemy == nil || enemy.unit == nil ||
		len(enemy.enemyDropAssetIDs) == 0 {
		return
	}

	// 同一个单位键异常重复时只保留一次. 角色和战宠的单位键不同, 随后虽然
	// 都映射到同一主人, 仍会各占一个候选位置, 与8.5的Entry数组一致.
	seenAttackerUnits := make(map[string]struct{}, len(attackerKeys))
	owners := make([]*combatUnitRuntimeState, 0, len(attackerKeys))
	for _, attackerKey := range attackerKeys {
		keyString := combatUnitKeyMapKey(attackerKey)
		if _, exists := seenAttackerUnits[keyString]; exists {
			continue
		}
		seenAttackerUnits[keyString] = struct{}{}

		attacker := r.stateByKey(attackerKey)
		if attacker == nil || attacker.unit == nil ||
			attacker.unit.GetCamp() == enemy.unit.GetCamp() {
			continue
		}
		key := attacker.unit.GetKey()
		if key.GetAid() == 0 || key.GetCharacterUuid() == 0 {
			continue
		}
		owner := r.stateByKey(&pb.CombatUnitKey{
			Aid:           key.GetAid(),
			CharacterUuid: key.GetCharacterUuid(),
		})
		if owner == nil || owner.unit == nil || !combatUnitIsPlayerCharacter(owner.unit) {
			continue
		}
		owners = append(owners, owner)
	}
	if len(owners) == 0 {
		return
	}

	for _, assetID := range enemy.enemyDropAssetIDs {
		ownerIndex := r.random.rangeInt(0, int64(len(owners)-1))
		owner := owners[ownerIndex]
		if len(owner.battleDropAssetIDs) < combatPVEGetItemMax {
			owner.battleDropAssetIDs = append(owner.battleDropAssetIDs, assetID)
			continue
		}
		if r.random.rangeInt(0, 1) == 0 {
			continue
		}
		replaceIndex := r.random.rangeInt(0, combatPVEGetItemMax-1)
		owner.battleDropAssetIDs[replaceIndex] = assetID
	}

	// 旧服在分配时把ITEM实例从敌人十格中移除. 清空敌方运行态既表达相同
	// 所有权转移, 也确保未来即使误改一次性死亡标志也无法重复分配.
	enemy.enemyDropAssetIDs = nil
}

// addPVEDuelPointDefeatProfit复刻当前有效#ifndef DANTAI分支的
// BATTLE_AddDuelPoint, 并限定为当前单角色、可选一只战宠的PVE范围.
//
// 旧函数把攻击列表中的玩家宠物转换成主人后尝试去重, 但实际比较使用了错误
// 下标. 在当前最多“角色+自己的战宠”两项攻击列表中, 两项都会保留并指向同一
// 角色: 敌人DP先除以2, 然后同一角色加两次; 除法结果不足1时每项又各保底1.
// 这里故意保留这个可确定的8.5结果, 不能用常规owner去重改变小DP合击奖励.
//
// DP战的死亡扫描还会处理玩家角色: 以开战DP的10%向零截断后累计为负值.
// 玩家宠物死亡没有DP数值变化, 但同样必须封存一次性死亡标记.
func (r *CombatRoom) addPVEDuelPointDefeatProfit(attackerKeys []*pb.CombatUnitKey) {
	if r == nil || r.battleStart == nil || len(attackerKeys) == 0 {
		return
	}
	for _, unit := range r.battleStart.GetUnitList() {
		defeated := r.stateByKey(unit.GetKey())
		if defeated == nil || defeated.unit == nil || defeated.alive ||
			defeated.escaped || defeated.defeatProfitProcessed {
			continue
		}
		defeated.defeatProfitProcessed = true

		if combatUnitIsPlayerCharacter(defeated.unit) {
			lost := int64(float64(defeated.characterDuelPoint) * 0.1)
			defeated.battleDuelPoint -= lost
			continue
		}
		if !defeated.pveEnemy {
			// 玩家宠物等非Enemy单位只完成CHAR_ISDIE等价标记, 不产生DP.
			continue
		}

		// 攻击列表按实际单位键去除异常重复键, 但角色和战宠是两个不同Entry,
		// 都必须各自转换成同一个主人并保留, 以复刻上方说明的旧下标错误.
		seenAttackerUnits := make(map[string]struct{}, len(attackerKeys))
		owners := make([]*combatUnitRuntimeState, 0, len(attackerKeys))
		for _, attackerKey := range attackerKeys {
			keyString := combatUnitKeyMapKey(attackerKey)
			if _, exists := seenAttackerUnits[keyString]; exists {
				continue
			}
			seenAttackerUnits[keyString] = struct{}{}

			attacker := r.stateByKey(attackerKey)
			if attacker == nil || attacker.unit == nil ||
				attacker.unit.GetCamp() == defeated.unit.GetCamp() {
				continue
			}
			key := attacker.unit.GetKey()
			if key.GetAid() == 0 || key.GetCharacterUuid() == 0 {
				continue
			}
			owner := r.stateByKey(&pb.CombatUnitKey{
				Aid:           key.GetAid(),
				CharacterUuid: key.GetCharacterUuid(),
			})
			if owner == nil || owner.unit == nil || !combatUnitIsPlayerCharacter(owner.unit) {
				continue
			}
			owners = append(owners, owner)
		}
		if len(owners) == 0 {
			continue
		}

		duelPoint := int64(float64(defeated.enemyDuelPoint) * 0.1)
		duelPoint /= int64(len(owners))
		if duelPoint <= 0 {
			duelPoint = 1
		}
		for _, owner := range owners {
			owner.battleDuelPoint += duelPoint
		}
	}
}

// battleSettlementIfFinished 根据双方存活规则生成全局胜负结果; nil 表示战斗尚未结束.
func (r *CombatRoom) battleSettlementIfFinished() *pb.CombatBattleSettlement {
	if r == nil {
		return nil
	}
	initiatorAlive := r.campBattleAlive(pb.CombatCamp_CombatCamp_Initiator)
	defenderAlive := r.campBattleAlive(pb.CombatCamp_CombatCamp_Defender)
	if initiatorAlive && defenderAlive {
		return nil
	}
	// 房间运行态只在这里冻结全局胜负. 经验及后续掉落必须等Account
	// actor收到最终结果后, 使用角色聚合根一次性持久化; 不能在房间actor
	// 中直接改档案, 也不能让客户端提交奖励数值.
	settlement := &pb.CombatBattleSettlement{}
	switch {
	case !initiatorAlive:
		settlement.BattleResult = pb.CombatBattleResult_CombatBattleResult_DefenderWin
	default:
		settlement.BattleResult = pb.CombatBattleResult_CombatBattleResult_InitiatorWin
	}
	return settlement
}

// campBattleAlive 判断阵营是否仍可继续战斗: 玩家阵营只看玩家角色, NPC 阵营检查全部单位.
func (r *CombatRoom) campBattleAlive(camp pb.CombatCamp) bool {
	hasPlayerCharacter := false
	for _, unit := range r.battleStart.GetUnitList() {
		if unit.GetCamp() == camp && combatUnitIsPlayerCharacter(unit) {
			hasPlayerCharacter = true
			break
		}
	}
	if hasPlayerCharacter {
		for _, unit := range r.battleStart.GetUnitList() {
			if unit.GetCamp() == camp && combatUnitIsPlayerCharacter(unit) && r.isAlive(unit.GetKey()) {
				return true
			}
		}
		return false
	}
	// NPC 阵营只有全部单位阵亡时才失败.
	for _, unit := range r.battleStart.GetUnitList() {
		if unit.GetCamp() == camp && r.isAlive(unit.GetKey()) {
			return true
		}
	}
	return false
}

func combatUnitIsPlayerCharacter(unit *pb.CombatUnit) bool {
	return combatUnitIsCharacter(unit) && unit.GetKey().GetAid() != 0 && unit.GetKey().GetCharacterUuid() != 0
}

// combatUnitIsCharacter 判断战斗单位是否为角色而非宠物.
func combatUnitIsCharacter(unit *pb.CombatUnit) bool {
	return unit != nil && unit.GetCharacterId() != 0 && unit.GetPetId() == 0
}

// cloneCombatUnitKey 复制单位键, 防止通知消息与运行期对象共享可变 protobuf 实例.
func cloneCombatUnitKey(key *pb.CombatUnitKey) *pb.CombatUnitKey {
	if key == nil {
		return nil
	}
	return &pb.CombatUnitKey{
		Aid:           key.GetAid(),
		CharacterUuid: key.GetCharacterUuid(),
		PetUuid:       key.GetPetUuid(),
	}
}

// combatUnitKeyEqual 比较两个单位键的账号、角色和宠物标识是否完全一致.
func combatUnitKeyEqual(a *pb.CombatUnitKey, b *pb.CombatUnitKey) bool {
	if a == nil || b == nil {
		return false
	}
	return a.GetAid() == b.GetAid() &&
		a.GetCharacterUuid() == b.GetCharacterUuid() &&
		a.GetPetUuid() == b.GetPetUuid()
}

// combatUnitKeyEmpty 判断单位键的全部标识字段是否均为 0.
func combatUnitKeyEmpty(key *pb.CombatUnitKey) bool {
	return key.GetAid() == 0 && key.GetCharacterUuid() == 0 && key.GetPetUuid() == 0
}

// combatUnitKeyMapKey 将单位键编码为战斗状态 map 使用的稳定字符串键.
func combatUnitKeyMapKey(key *pb.CombatUnitKey) string {
	if key == nil {
		return "0:0:0"
	}
	// 固定字段顺序并保留 0 值, 使角色, 玩家宠物和敌方临时宠物不会发生键语义混淆.
	return fmt.Sprintf("%d:%d:%d", key.GetAid(), key.GetCharacterUuid(), key.GetPetUuid())
}

// maxUint64 返回两个无符号整数中的较大值.
func maxUint64(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
