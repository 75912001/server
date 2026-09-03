package main

import (
	"context"
	"fmt"
	"sort"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xtimer "github.com/75912001/xlib/timer"
	"google.golang.org/protobuf/proto"
)

const (
	combatRoomActorCmdStart       xactor.CMD = 201
	combatRoomActorCmdRoundAction xactor.CMD = 202
	combatRoomActorCmdDetach      xactor.CMD = 203
	combatRoomActorCmdAddMember   xactor.CMD = 204
)

type combatRoomParticipantKey struct {
	aid           uint64
	characterUUID uint64
}

// combatRoomParticipant 保存房间向参战角色收发消息所需的不可变路由和可控单位快照.
type combatRoomParticipant struct {
	key             combatRoomParticipantKey
	account         *Account
	gateway         *Gateway
	playerCharacter *pb.CombatUnit
	playerPet       *pb.CombatUnit
}

// combatRoomParticipantAdmission 是 Account actor 完成档案读取后交给房间的完整入场快照.
// unitStates 与 participant 中的单位共享不可变 CombatUnit 指针, 由房间接收后独占运行态.
type combatRoomParticipantAdmission struct {
	participant *combatRoomParticipant
	unitStates  map[string]*combatUnitRuntimeState
}

type combatRoomUnitLeave struct {
	unitKeys []*pb.CombatUnitKey
	reason   pb.CombatUnitLeaveReason
}

type combatParticipantLeaveKind uint8

const (
	combatParticipantLeaveKindUnknown combatParticipantLeaveKind = iota
	combatParticipantLeaveKindEscape
	combatParticipantLeaveKindKnockback
)

// combatRoomFinishInput 封装 CombatRoom actor 交给 Account actor 的结算消息.
// enemyGroupID和battleReward是建房及结算值快照, result是接收角色独占副本;
// Account据此推进角色任务, 但不读取CombatRoom运行态.
type combatRoomFinishInput struct {
	characterUUID uint64
	combatRoom    *xactor.Actor[string]
	gateway       *Gateway
	result        *pb.CombatRoundResultNotify
	enemyGroupID  uint32
	battleReward  combatParticipantBattleReward
	rewardErr     error
}

// combatRoomParticipantLeaveInput 封装仍需收到当前回合结果、但不会继续留在房间的参与者.
type combatRoomParticipantLeaveInput struct {
	characterUUID uint64
	combatRoom    *xactor.Actor[string]
	gateway       *Gateway
	result        *pb.CombatRoundResultNotify
	kind          combatParticipantLeaveKind
}

// CombatRoom 是一场战斗的权威运行实例.
// 所有可变字段只允许由房间 actor 读写; character 只持有 actor 指针, 仅通过消息投递交互.
type CombatRoom struct {
	actor      *xactor.Actor[string]
	roundTimer *xtimer.Second

	battleID string
	// enemyGroupID在PVE建房时由服务端配置冻结, 仅用于战斗胜利后的任务事件.
	enemyGroupID uint32
	round        uint32

	participantOrder       []combatRoomParticipantKey
	participants           map[combatRoomParticipantKey]*combatRoomParticipant
	pendingUnitLeaves      []combatRoomUnitLeave
	roundParticipantLeaves map[combatRoomParticipantKey]combatParticipantLeaveKind

	battleStart   *pb.CombatBattleStartNotify
	enemyUnits    []*pb.CombatUnit
	unitStates    map[string]*combatUnitRuntimeState
	playerActions map[string]*combatAction
	random        *combatRandom
	// pveDuelPointBattle对应BattleArray.dpbattle. 建房时只要任一PVE敌人
	// 的DUELPOINT大于0就锁定为true, 整场改走DP死亡归属和结束结算,
	// 不再发放普通EXP、战斗石币或掉落. 该模式不会在中途随敌人死亡而切回.
	pveDuelPointBattle bool
}

// GCombatRoomMgr 保存当前 online 进程内仍存活的战斗房间.
var GCombatRoomMgr = &CombatRoomMgr{
	rooms: xmap.NewMapMutexMgr[string, *CombatRoom](),
}

// CombatRoomMgr 管理当前 online 进程内存活的战斗房间, key 为 battle ID.
type CombatRoomMgr struct {
	rooms *xmap.MapMutexMgr[string, *CombatRoom]
}

// newCombatRoom 使用队长入场快照完整构造并注册房间, actor 启动后仍可在首回合前追加成功队员.
func newCombatRoom(
	battleID string,
	enemyGroupID uint32,
	participant *combatRoomParticipant,
	battleStart *pb.CombatBattleStartNotify,
	enemyUnits []*pb.CombatUnit,
	unitStates map[string]*combatUnitRuntimeState,
) (*CombatRoom, error) {
	seed, err := newCombatRandomSeed()
	if err != nil {
		return nil, fmt.Errorf("combat random seed create failed: %w", err)
	}
	return newCombatRoomWithSeed(battleID, enemyGroupID, participant, battleStart, enemyUnits, unitStates, seed)
}

// newCombatRoomWithSeed 使用指定种子构造房间, 只供确定性测试和newCombatRoom生产入口复用.
func newCombatRoomWithSeed(
	battleID string,
	enemyGroupID uint32,
	participant *combatRoomParticipant,
	battleStart *pb.CombatBattleStartNotify,
	enemyUnits []*pb.CombatUnit,
	unitStates map[string]*combatUnitRuntimeState,
	seed uint64,
) (*CombatRoom, error) {
	if enemyGroupID == 0 {
		return nil, fmt.Errorf("combat enemy group id is zero")
	}
	if err := validateCombatRoomCreateArguments(battleID, participant, battleStart); err != nil {
		return nil, err
	}
	room := &CombatRoom{
		battleID:         battleID,
		enemyGroupID:     enemyGroupID,
		round:            1,
		participantOrder: []combatRoomParticipantKey{participant.key},
		participants: map[combatRoomParticipantKey]*combatRoomParticipant{
			participant.key: participant,
		},
		battleStart:   battleStart,
		enemyUnits:    enemyUnits,
		unitStates:    unitStates,
		playerActions: make(map[string]*combatAction),
		random:        newCombatRandom(seed),
	}
	room.pveDuelPointBattle = combatRoomIsPVEDuelPointBattle(room.unitStates)
	if err := room.validatePhysicalState(); err != nil {
		return nil, err
	}
	// 生产seed只用于初始化本房间内存中的确定性随机流, 不进入日志、协议、持久化数据
	// 或回放档案. 一旦完整seed出现在日志中, 能读取日志的人就可以结合公开的PCG32算法
	// 重建本场后续命中、伤害、反击和逃跑抽值, 因而即使是Debug级别也不能输出seed数值.
	// 这里保留battle ID和算法版本, 只用于确认房间已经初始化预期的随机实现; 这两项信息
	// 不足以反推出随机流状态. 指定seed的构造入口仍保留给确定性测试, 但不会改变该日志边界.
	xlog.GLog.Debugf("combat deterministic random initialized battle:%s algorithm:pcg32-xsh-rr-v1", battleID)
	room.actor = xactor.NewActor[string](battleID, nil, room.behavior)
	if !GCombatRoomMgr.rooms.AddIfNotExist(battleID, room) {
		return nil, fmt.Errorf("combat room already exists battle:%s", battleID)
	}
	room.actor.Start()
	return room, nil
}

// combatRoomIsPVEDuelPointBattle复刻BATTLE_Create中对BattleArray.dpbattle的锁定.
//
// 敌人的DUELPOINT只有严格大于0才会把整场切换到DP流程. 0和-1都不会单独
// 开启DP模式; 但一旦同场任一敌人开启, 其他0或-1敌人也必须随整场继续走
// BATTLE_AddDuelPoint/BATTLE_GetDuelPoint, 不能在死亡或结算时按单个敌人切回
// EXP、石币和掉落流程. 这里只读取建房时已经冻结的服务器敌人运行态, 不接收
// 客户端字段, 也不把玩家角色或战宠误识别成PVE敌人.
func combatRoomIsPVEDuelPointBattle(states map[string]*combatUnitRuntimeState) bool {
	for _, state := range states {
		if state != nil && state.pveEnemy && state.enemyDuelPoint > 0 {
			return true
		}
	}
	return false
}

// validateCombatRoomCreateArguments 校验房间构造所需的队长路由和角色快照; 玩家战宠允许为空.
func validateCombatRoomCreateArguments(battleID string, participant *combatRoomParticipant, battleStart *pb.CombatBattleStartNotify) error {
	if battleID == "" || participant == nil || participant.account == nil || participant.gateway == nil || participant.key.aid == 0 || participant.key.characterUUID == 0 || participant.playerCharacter == nil || battleStart == nil {
		return fmt.Errorf("combat room create argument invalid")
	}
	return nil
}

// postCombatRoomStart 请求房间向参与者推送开战快照并开始首回合.
func postCombatRoomStart(combatRoom *xactor.Actor[string]) {
	if combatRoom == nil {
		return
	}
	combatRoom.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdStart))
}

// addCombatRoomParticipantSync 在首回合启动前由房间 actor 分配紧凑站位并接收入场快照.
func addCombatRoomParticipantSync(combatRoom *xactor.Actor[string], admission combatRoomParticipantAdmission) error {
	if combatRoom == nil {
		return fmt.Errorf("combat room actor is nil")
	}
	resp, err := combatRoom.SendMsgSync(xactor.NewMsg(context.Background(), combatRoomActorCmdAddMember, admission))
	if err != nil {
		return err
	}
	if addErr, ok := resp.(error); ok {
		return addErr
	}
	return nil
}

// postCombatRoomRoundAction 将指定参与者的单位动作投递给房间 actor.
func postCombatRoomRoundAction(combatRoom *xactor.Actor[string], key combatRoomParticipantKey, gateway *Gateway, req *pb.CombatRoundActionReq) {
	if combatRoom == nil {
		return
	}
	combatRoom.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdRoundAction, key, gateway, req))
}

// postCombatRoomDetach 通知房间记录角色下线或运行态清理产生的外部脱离, 与技能逃跑分开处理.
func postCombatRoomDetach(combatRoom *xactor.Actor[string], key combatRoomParticipantKey) {
	if combatRoom == nil {
		return
	}
	combatRoom.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdDetach, key))
}

func (r *CombatRoom) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if r.roundTimer != nil && event.ISwitch.IsOn() {
				if err := event.ICallBack.Execute(); err != nil {
					xlog.GLog.Warnf("combat room event callback failed battle:%s err:%v", r.battleID, err)
				}
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		if msg.Cmd == combatRoomActorCmdRoundAction && r.roundTimer == nil {
			continue
		}
		switch msg.Cmd {
		case combatRoomActorCmdStart:
			r.start()
		case combatRoomActorCmdRoundAction:
			if len(msg.Args) != 3 {
				continue
			}
			key, keyOK := msg.Args[0].(combatRoomParticipantKey)
			gateway, gatewayOK := msg.Args[1].(*Gateway)
			req, reqOK := msg.Args[2].(*pb.CombatRoundActionReq)
			if keyOK && gatewayOK && reqOK {
				r.onCombatRoundActionReq(key, gateway, req)
			}
		case combatRoomActorCmdDetach:
			if len(msg.Args) != 1 {
				continue
			}
			if key, ok := msg.Args[0].(combatRoomParticipantKey); ok {
				r.detachParticipant(key)
			}
		case combatRoomActorCmdAddMember:
			if len(msg.Args) != 1 {
				resp = fmt.Errorf("combat room add participant argument invalid")
				continue
			}
			admission, ok := msg.Args[0].(combatRoomParticipantAdmission)
			if !ok {
				resp = fmt.Errorf("combat room add participant type invalid")
				continue
			}
			resp = r.addParticipant(admission)
		}
	}
	return r.behavior, resp, nil
}

func (r *CombatRoom) participant(key combatRoomParticipantKey) *combatRoomParticipant {
	if r == nil {
		return nil
	}
	return r.participants[key]
}

func (r *CombatRoom) markParticipantLeaving(key combatRoomParticipantKey, kind combatParticipantLeaveKind) {
	if r == nil || kind == combatParticipantLeaveKindUnknown || r.participant(key) == nil {
		return
	}
	if r.roundParticipantLeaves == nil {
		r.roundParticipantLeaves = make(map[combatRoomParticipantKey]combatParticipantLeaveKind)
	}
	if r.roundParticipantLeaves[key] == combatParticipantLeaveKindUnknown {
		r.roundParticipantLeaves[key] = kind
	}
}

func (r *CombatRoom) allParticipantsLeavingThisRound() bool {
	if r == nil || len(r.participants) == 0 || len(r.roundParticipantLeaves) != len(r.participants) {
		return false
	}
	for key := range r.participants {
		if r.roundParticipantLeaves[key] == combatParticipantLeaveKindUnknown {
			return false
		}
	}
	return true
}

func (r *CombatRoom) finalizeRoundParticipantLeaves() {
	if r == nil || len(r.roundParticipantLeaves) == 0 {
		return
	}
	for key := range r.roundParticipantLeaves {
		participant := r.participant(key)
		if participant == nil {
			continue
		}
		r.removeParticipant(participant)
		delete(r.participants, key)
	}
	r.roundParticipantLeaves = nil
}

// addParticipant 只接受尚未启动首回合的成功绑定成员, 并按成功顺序压缩角色和战宠站位.
func (r *CombatRoom) addParticipant(admission combatRoomParticipantAdmission) error {
	if r == nil || r.roundTimer != nil || r.battleStart == nil || r.participants == nil || r.unitStates == nil {
		return fmt.Errorf("combat room is not accepting participants")
	}
	participant := admission.participant
	if err := validateCombatRoomParticipantAdmission(participant, admission.unitStates); err != nil {
		return err
	}
	if len(r.participantOrder) >= combatCampRowPositionCount {
		return fmt.Errorf("combat room participant limit reached")
	}
	if r.participant(participant.key) != nil {
		return fmt.Errorf("combat room participant already exists aid:%d character:%d", participant.key.aid, participant.key.characterUUID)
	}
	for unitKey := range admission.unitStates {
		if r.unitStates[unitKey] != nil {
			return fmt.Errorf("combat room participant unit already exists: %s", unitKey)
		}
	}

	position := uint32(len(r.participantOrder))
	participant.playerCharacter.Position = position
	if participant.playerPet != nil {
		participant.playerPet.Position = position + combatCampRowPositionCount
	}

	insertIndex := len(r.battleStart.UnitList)
	for index, unit := range r.battleStart.UnitList {
		if unit != nil && unit.GetCamp() == pb.CombatCamp_CombatCamp_Defender {
			insertIndex = index
			break
		}
	}
	oldUnitList := r.battleStart.UnitList
	playerUnits := make([]*pb.CombatUnit, 0, insertIndex+2)
	playerUnits = append(playerUnits, oldUnitList[:insertIndex]...)
	playerUnits = append(playerUnits, participant.playerCharacter)
	if participant.playerPet != nil {
		playerUnits = append(playerUnits, participant.playerPet)
	}
	sort.SliceStable(playerUnits, func(left int, right int) bool {
		return playerUnits[left].GetPosition() < playerUnits[right].GetPosition()
	})
	unitList := make([]*pb.CombatUnit, 0, len(oldUnitList)+2)
	unitList = append(unitList, playerUnits...)
	unitList = append(unitList, oldUnitList[insertIndex:]...)
	r.battleStart.UnitList = unitList
	r.participantOrder = append(r.participantOrder, participant.key)
	r.participants[participant.key] = participant
	for unitKey, state := range admission.unitStates {
		r.unitStates[unitKey] = state
	}
	if err := r.validatePhysicalState(); err != nil {
		for unitKey := range admission.unitStates {
			delete(r.unitStates, unitKey)
		}
		delete(r.participants, participant.key)
		r.participantOrder = r.participantOrder[:len(r.participantOrder)-1]
		r.battleStart.UnitList = oldUnitList
		return err
	}
	return nil
}

func validateCombatRoomParticipantAdmission(participant *combatRoomParticipant, unitStates map[string]*combatUnitRuntimeState) error {
	if participant == nil || participant.account == nil || participant.gateway == nil || participant.key.aid == 0 || participant.key.characterUUID == 0 || participant.playerCharacter == nil {
		return fmt.Errorf("combat room participant admission invalid")
	}
	characterKey := participant.playerCharacter.GetKey()
	if characterKey.GetAid() != participant.key.aid || characterKey.GetCharacterUuid() != participant.key.characterUUID || characterKey.GetPetUuid() != 0 || participant.playerCharacter.GetCamp() != pb.CombatCamp_CombatCamp_Initiator {
		return fmt.Errorf("combat room participant character identity invalid")
	}
	units := []*pb.CombatUnit{participant.playerCharacter}
	if participant.playerPet != nil {
		petKey := participant.playerPet.GetKey()
		if petKey.GetAid() != participant.key.aid || petKey.GetCharacterUuid() != participant.key.characterUUID || petKey.GetPetUuid() == 0 || participant.playerPet.GetCamp() != pb.CombatCamp_CombatCamp_Initiator {
			return fmt.Errorf("combat room participant pet identity invalid")
		}
		units = append(units, participant.playerPet)
	}
	if len(unitStates) != len(units) {
		return fmt.Errorf("combat room participant state count invalid")
	}
	for _, unit := range units {
		state := unitStates[combatUnitKeyMapKey(unit.GetKey())]
		if state == nil || state.unit != unit || state.unit.GetAttribute() == nil || state.unit.GetAttribute().GetLevel() == 0 || !state.alive || state.hp == 0 || state.hp != state.maxHP {
			return fmt.Errorf("combat room participant state invalid")
		}
	}
	return nil
}

func (r *CombatRoom) start() {
	if r == nil || r.roundTimer != nil || r.battleStart == nil {
		return
	}
	r.beginCombatRound()
	if r.roundTimer == nil {
		return
	}
	for _, key := range r.participantOrder {
		participant := r.participant(key)
		if participant == nil {
			continue
		}
		participant.account.sendClientRes(participant.gateway, uint32(pb.MsgID_CombatBattleStartNotify_CMD), 0, r.battleStart)
	}
}

// detachParticipant 处理外部脱离: 立即从本回合和奖励集合移除成员, 但让其余参与者继续战斗.
func (r *CombatRoom) detachParticipant(key combatRoomParticipantKey) {
	participant := r.participant(key)
	if participant == nil {
		return
	}
	unitKeys := r.removeParticipant(participant)
	delete(r.participants, key)
	if len(unitKeys) > 0 {
		r.pendingUnitLeaves = append(r.pendingUnitLeaves, combatRoomUnitLeave{
			unitKeys: unitKeys,
			reason:   pb.CombatUnitLeaveReason_CombatUnitLeaveReason_Detached,
		})
	}
	if len(r.participants) == 0 {
		r.closeCombatRoom()
		return
	}
	if r.roundTimer != nil && (r.battleSettlementIfFinished() != nil || r.playerActionsReady()) {
		r.completeCombatRound(r.collectedPlayerActions())
	}
}

func (r *CombatRoom) removeParticipant(participant *combatRoomParticipant) []*pb.CombatUnitKey {
	if participant == nil {
		return nil
	}
	unitKeys := make([]*pb.CombatUnitKey, 0, 2)
	for _, unit := range []*pb.CombatUnit{participant.playerCharacter, participant.playerPet} {
		if unit == nil {
			continue
		}
		unitKeys = append(unitKeys, cloneCombatUnitKey(unit.GetKey()))
		delete(r.playerActions, combatUnitKeyMapKey(unit.GetKey()))
		state := r.stateByKey(unit.GetKey())
		if state == nil {
			continue
		}
		state.hp = 0
		state.alive = false
		state.guard = false
		state.charge = nil
		state.battleExperience = 0
		state.battleDuelPoint = 0
		state.battleDropAssetIDs = nil
	}
	return unitKeys
}

// takePendingUnitLeaveEvents 把 actor 顺序内已经发生的外部脱离转换为本回合最前面的协议事件.
func (r *CombatRoom) takePendingUnitLeaveEvents() []*pb.CombatEvent {
	if r == nil || len(r.pendingUnitLeaves) == 0 {
		return nil
	}
	events := make([]*pb.CombatEvent, 0, len(r.pendingUnitLeaves))
	for _, leave := range r.pendingUnitLeaves {
		if len(leave.unitKeys) == 0 {
			continue
		}
		events = append(events, &pb.CombatEvent{
			ActionStepList: []*pb.CombatActionStep{{
				Cause: pb.CombatActionCause_CombatActionCause_Passive,
				EffectList: []*pb.CombatEffect{{
					AffectedUnitKeyList: cloneCombatUnitKeyList(leave.unitKeys),
					Detail: &pb.CombatEffect_UnitLeave{UnitLeave: &pb.CombatUnitLeaveDetail{
						Reason: leave.reason,
					}},
				}},
			}},
		})
	}
	r.pendingUnitLeaves = nil
	return events
}

// combatRoundResultForRecipient 为单个接收角色创建独立战报, 防止个人结算在参与者之间共享.
func combatRoundResultForRecipient(result *pb.CombatRoundResultNotify, characterUUID uint64) *pb.CombatRoundResultNotify {
	if result == nil {
		return nil
	}
	participantResult := proto.Clone(result).(*pb.CombatRoundResultNotify)
	participantResult.RecipientCharacterUuid = characterUUID
	return participantResult
}

// sendRoundResultAndFinalizeParticipantLeaves 投递普通回合战报, 并让本回合主动离场者只收到这一份战报后退出房间.
func (r *CombatRoom) sendRoundResultAndFinalizeParticipantLeaves(result *pb.CombatRoundResultNotify) {
	if r == nil || result == nil {
		return
	}
	for _, key := range r.participantOrder {
		participant := r.participant(key)
		if participant == nil || participant.account == nil {
			continue
		}
		participantResult := combatRoundResultForRecipient(result, participant.key.characterUUID)
		leaveKind := r.roundParticipantLeaves[key]
		if leaveKind == combatParticipantLeaveKindUnknown {
			participant.account.sendClientRes(participant.gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Success.Code(), participantResult)
			continue
		}
		participantResult.Settlement = nil
		if err := participant.account.PostCombatRoomParticipantLeftSync(combatRoomParticipantLeaveInput{
			characterUUID: participant.key.characterUUID,
			combatRoom:    r.actor,
			gateway:       participant.gateway,
			result:        participantResult,
			kind:          leaveKind,
		}); err != nil {
			xlog.GLog.Errorf("combat room account leave sync failed battle:%s aid:%d character:%d err:%v", r.battleID, participant.key.aid, participant.key.characterUUID, err)
		}
	}
	r.finalizeRoundParticipantLeaves()
}

// finishCombat 在回合定时器取消后生成各参与者的不可变奖励输入, 同步清理匹配的 Account actor 指针并投递最终战报, 再注销和停止房间.
func (r *CombatRoom) finishCombat(result *pb.CombatRoundResultNotify) {
	if r == nil || r.roundTimer == nil {
		return
	}
	if result == nil || result.GetSettlement() == nil {
		xlog.GLog.Errorf("combat room finish result missing settlement battle:%s", r.battleID)
		return
	}
	if result.GetSettlement().GetBattleResult() == pb.CombatBattleResult_CombatBattleResult_Unknown {
		xlog.GLog.Errorf("combat room finish result missing battle result battle:%s", r.battleID)
		return
	}
	r.clearRoundTimer()
	for _, key := range r.participantOrder {
		participant := r.participant(key)
		if participant == nil || participant.account == nil {
			continue
		}
		participantResult := combatRoundResultForRecipient(result, participant.key.characterUUID)
		if leaveKind := r.roundParticipantLeaves[key]; leaveKind != combatParticipantLeaveKindUnknown {
			participantResult.Settlement = nil
			if err := participant.account.PostCombatRoomParticipantLeftSync(combatRoomParticipantLeaveInput{
				characterUUID: participant.key.characterUUID,
				combatRoom:    r.actor,
				gateway:       participant.gateway,
				result:        participantResult,
				kind:          leaveKind,
			}); err != nil {
				xlog.GLog.Errorf("combat room account leave during finish sync failed battle:%s aid:%d character:%d err:%v", r.battleID, participant.key.aid, participant.key.characterUUID, err)
			}
			continue
		}
		battleReward, rewardErr := r.playerCombatBattleReward(
			participant.key,
			result.GetSettlement().GetBattleResult(),
		)
		if err := participant.account.PostCombatRoomFinishedSync(combatRoomFinishInput{
			characterUUID: participant.key.characterUUID,
			combatRoom:    r.actor,
			gateway:       participant.gateway,
			result:        participantResult,
			enemyGroupID:  r.enemyGroupID,
			battleReward:  battleReward,
			rewardErr:     rewardErr,
		}); err != nil {
			xlog.GLog.Errorf("combat room account finish sync failed battle:%s aid:%d character:%d err:%v", r.battleID, participant.key.aid, participant.key.characterUUID, err)
		}
	}
	r.finalizeRoundParticipantLeaves()
	r.closeCombatRoom()
}

// closeCombatRoom 注销房间并释放全部战斗运行态; 调用者必须先完成需要读取房间状态的结算计算.
func (r *CombatRoom) closeCombatRoom() {
	if r == nil {
		return
	}
	r.clearRoundTimer()
	if current, ok := GCombatRoomMgr.rooms.Find(r.battleID); ok && current == r {
		GCombatRoomMgr.rooms.Del(r.battleID)
	}
	r.participantOrder = nil
	r.participants = nil
	r.pendingUnitLeaves = nil
	r.roundParticipantLeaves = nil
	r.battleStart = nil
	r.enemyGroupID = 0
	r.enemyUnits = nil
	r.unitStates = nil
	r.playerActions = nil
	if r.actor != nil {
		r.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}
}
