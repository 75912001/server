package main

import (
	"context"
	"fmt"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	xtimer "github.com/75912001/xlib/timer"
	"google.golang.org/protobuf/proto"
)

const (
	combatRoomActorCmdStart       xactor.CMD = 201
	combatRoomActorCmdRoundAction xactor.CMD = 202
	combatRoomActorCmdLeave       xactor.CMD = 203
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

// CombatRoom 是一场战斗的权威运行实例.
// 所有可变字段只允许由房间 actor 读写; character 只持有房间指针并调用 PostXXX 发送消息.
type CombatRoom struct {
	actor      *xactor.Actor[string]
	roundTimer *xtimer.Second

	battleID string
	round    uint32

	participantOrder []combatRoomParticipantKey
	participants     map[combatRoomParticipantKey]*combatRoomParticipant

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

// newCombatRoom 完整构造并注册房间, 注册成功后启动房间 actor 再返回.
func newCombatRoom(
	battleID string,
	participant *combatRoomParticipant,
	battleStart *pb.CombatBattleStartNotify,
	enemyUnits []*pb.CombatUnit,
	unitStates map[string]*combatUnitRuntimeState,
) (*CombatRoom, error) {
	seed, err := newCombatRandomSeed()
	if err != nil {
		return nil, fmt.Errorf("combat random seed create failed: %w", err)
	}
	return newCombatRoomWithSeed(battleID, participant, battleStart, enemyUnits, unitStates, seed)
}

// newCombatRoomWithSeed 使用指定种子构造房间, 只供确定性测试和newCombatRoom生产入口复用.
func newCombatRoomWithSeed(
	battleID string,
	participant *combatRoomParticipant,
	battleStart *pb.CombatBattleStartNotify,
	enemyUnits []*pb.CombatUnit,
	unitStates map[string]*combatUnitRuntimeState,
	seed uint64,
) (*CombatRoom, error) {
	if err := validateCombatRoomCreateArguments(battleID, participant, battleStart); err != nil {
		return nil, err
	}
	room := &CombatRoom{
		battleID:         battleID,
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

// validateCombatRoomCreateArguments 校验房间构造所需的参与者路由和角色快照; 玩家战宠允许为空.
func validateCombatRoomCreateArguments(battleID string, participant *combatRoomParticipant, battleStart *pb.CombatBattleStartNotify) error {
	if battleID == "" || participant == nil || participant.account == nil || participant.gateway == nil || participant.key.aid == 0 || participant.key.characterUUID == 0 || participant.playerCharacter == nil || battleStart == nil {
		return fmt.Errorf("combat room create argument invalid")
	}
	return nil
}

// PostStart 请求房间向参与者推送开战快照并开始首回合.
func (r *CombatRoom) PostStart() {
	if r == nil || r.actor == nil {
		return
	}
	r.actor.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdStart))
}

// PostRoundAction 将指定参与者的单位动作投递给房间 actor.
func (r *CombatRoom) PostRoundAction(key combatRoomParticipantKey, gateway *Gateway, req *pb.CombatRoundActionReq) {
	if r == nil || r.actor == nil {
		return
	}
	r.actor.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdRoundAction, key, gateway, req))
}

// PostLeave 通知房间移除指定参与者及其单位, 再按剩余玩家角色判断战斗是否结束.
func (r *CombatRoom) PostLeave(key combatRoomParticipantKey) {
	if r == nil || r.actor == nil {
		return
	}
	r.actor.SendMsg(xactor.NewMsg(context.Background(), combatRoomActorCmdLeave, key))
}

func (r *CombatRoom) behavior(messages ...any) (xactor.Behavior, any, error) {
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
		if msg.Cmd != combatRoomActorCmdStart && r.roundTimer == nil {
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
		case combatRoomActorCmdLeave:
			if len(msg.Args) != 1 {
				continue
			}
			if key, ok := msg.Args[0].(combatRoomParticipantKey); ok {
				r.onParticipantLeave(key)
			}
		}
	}
	return r.behavior, nil, nil
}

func (r *CombatRoom) participant(key combatRoomParticipantKey) *combatRoomParticipant {
	if r == nil {
		return nil
	}
	return r.participants[key]
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

// onParticipantLeave 移除离线参与者及其单位; 只要玩家阵营仍有存活角色, 战斗继续.
func (r *CombatRoom) onParticipantLeave(key combatRoomParticipantKey) {
	participant := r.participant(key)
	if participant == nil {
		return
	}
	leavingCamp := participant.playerCharacter.GetCamp()
	r.removeParticipant(participant)
	delete(r.participants, key)

	if r.battleSettlementIfFinished() != nil {
		result := &pb.CombatRoundResultNotify{
			BattleId:   r.battleID,
			Round:      r.round,
			Settlement: r.escapeSettlement(leavingCamp),
		}
		r.finishCombat(result)
		return
	}
	if r.playerActionsReady() {
		r.completeCombatRound(r.collectedPlayerActions())
	}
}

func (r *CombatRoom) removeParticipant(participant *combatRoomParticipant) {
	if participant == nil {
		return
	}
	for _, unit := range []*pb.CombatUnit{participant.playerCharacter, participant.playerPet} {
		if unit == nil {
			continue
		}
		delete(r.playerActions, combatUnitKeyMapKey(unit.GetKey()))
		state := r.stateByKey(unit.GetKey())
		if state == nil {
			continue
		}
		state.hp = 0
		state.alive = false
		state.guard = false
	}
}

func (r *CombatRoom) escapeSettlement(loserCamp pb.CombatCamp) *pb.CombatBattleSettlement {
	battleResult := pb.CombatBattleResult_CombatBattleResult_InitiatorWin
	if loserCamp == pb.CombatCamp_CombatCamp_Initiator {
		battleResult = pb.CombatBattleResult_CombatBattleResult_DefenderWin
	}
	return &pb.CombatBattleSettlement{
		BattleResult: battleResult,
	}
}

// finishCombat 在回合定时器取消后同步清理各 Account 引用并投递最终战报, 再注销和停止房间.
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
		participantResult := proto.Clone(result).(*pb.CombatRoundResultNotify)
		if err := participant.account.PostCombatRoomFinishedSync(participant.key.characterUUID, r, participant.gateway, participantResult); err != nil {
			xlog.GLog.Errorf("combat room account finish sync failed battle:%s aid:%d character:%d err:%v", r.battleID, participant.key.aid, participant.key.characterUUID, err)
		}
	}
	if current, ok := GCombatRoomMgr.rooms.Find(r.battleID); ok && current == r {
		GCombatRoomMgr.rooms.Del(r.battleID)
	}
	r.participantOrder = nil
	r.participants = nil
	r.battleStart = nil
	r.enemyUnits = nil
	r.unitStates = nil
	r.playerActions = nil
	r.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
}
