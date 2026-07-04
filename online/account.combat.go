package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xtimer "github.com/75912001/xlib/timer"
	"google.golang.org/protobuf/proto"
)

const (
	autoEncounterInterval = 5 * time.Second
	combatRoundDuration   = 100 * time.Second

	initiatorCharacterPosition = 0
	initiatorPetPosition       = 5

	petSkillStandby      = 9000001
	petSkillDefense      = 9000003
	petSkillDoubleAttack = 9000005
	petSkillTripleAttack = 9000006
)

type accountCombatState struct {
	battleID   uint64
	round      uint32
	deadlineMs int64

	gateway         *Gateway
	battleStart     *pb.CombatBattleStart
	playerCharacter *pb.CombatUnit
	playerPet       *pb.CombatUnit
	enemyUnits      []*pb.CombatUnit
	unitStates      map[string]*combatUnitRuntimeState
	playerActions   map[string]*pb.CombatActionIntent
}

type combatUnitRuntimeState struct {
	unit         *pb.CombatUnit
	hp           uint64
	maxHP        uint64
	alive        bool
	guard        bool
	statusStates []*pb.CombatStatusState
}

func (p *Account) onAutoEncounterSetReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.AutoEncounterSetReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_AutoEncounterSetRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	if req.GetEnabled() {
		character, _, err := p.currentBattleCharacterAndPet()
		if err != nil {
			p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_AutoEncounterSetRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
		if _, err := p.characterSceneID(character); err != nil {
			p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_AutoEncounterSetRes_CMD), xerror.FailedPrecondition.Code())
			return
		}
	}

	p.autoEncounterEnabled = req.GetEnabled()
	if p.autoEncounterEnabled {
		if p.combatState == nil {
			p.restartAutoEncounterTimer(gateway)
		}
	} else {
		p.clearAutoEncounterTimer()
	}

	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_AutoEncounterSetRes_CMD), xerror.Success.Code(),
		&pb.AutoEncounterSetRes{
			Enabled:           p.autoEncounterEnabled,
			ServerTimestampMs: time.Now().UnixMilli(),
		},
	)
}

func (p *Account) onCombatRoundActionReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	if p.combatState == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	var req pb.CombatRoundActionReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if req.GetBattleId() != p.combatState.battleID || req.GetRound() != p.combatState.round {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}

	action, err := p.playerUnitAction(&req)
	if err != nil {
		xlog.GLog.Warnf("invalid combat action aid:%d battle:%d round:%d err:%v", p.aid, req.GetBattleId(), req.GetRound(), err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	actionKey := combatUnitKeyMapKey(action.GetUnitKey())
	if _, ok := p.combatState.playerActions[actionKey]; ok {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	p.combatState.playerActions[actionKey] = action

	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CombatRoundActionRes_CMD), xerror.Success.Code(),
		&pb.CombatRoundActionRes{
			BattleId: p.combatState.battleID,
			Round:    p.combatState.round,
			UnitKey:  cloneCombatUnitKey(action.GetUnitKey()),
		},
	)
	p.sendCombatRoundReadyNotify()
	if p.playerActionsReady() {
		p.completeCombatRound(p.collectedPlayerActions())
	}
}

func (p *Account) restartAutoEncounterTimer(gateway *Gateway) {
	p.clearAutoEncounterTimer()
	if xtimer.GTimer == nil {
		return
	}
	p.autoEncounterTimerSeq++
	timerSeq := p.autoEncounterTimerSeq
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if timerSeq != p.autoEncounterTimerSeq {
			return nil
		}
		p.autoEncounterTimer = nil
		p.onAutoEncounterTimer(gateway)
		return nil
	})
	p.autoEncounterTimer = xtimer.GTimer.AddSecond(cb, time.Now().Add(autoEncounterInterval).Unix(), p.actor)
}

func (p *Account) clearAutoEncounterTimer() {
	p.autoEncounterTimerSeq++
	if xtimer.GTimer != nil && p.autoEncounterTimer != nil {
		xtimer.GTimer.DelSecond(p.autoEncounterTimer)
	}
	p.autoEncounterTimer = nil
}

func (p *Account) onAutoEncounterTimer(gateway *Gateway) {
	if !p.autoEncounterEnabled {
		return
	}
	if p.combatState != nil {
		return
	}
	if err := p.startAutoEncounterBattle(gateway); err != nil {
		xlog.GLog.Warnf("auto encounter failed aid:%d err:%v", p.aid, err)
		p.autoEncounterEnabled = false
		p.clearAutoEncounterTimer()
	}
}

func (p *Account) startAutoEncounterBattle(gateway *Gateway) error {
	if gateway == nil {
		return fmt.Errorf("gateway is nil")
	}
	character, battlePet, err := p.currentBattleCharacterAndPet()
	if err != nil {
		return err
	}
	sceneID, err := p.characterSceneID(character)
	if err != nil {
		return err
	}
	enemyGroup, err := p.randomSceneEnemyGroup(sceneID)
	if err != nil {
		return err
	}

	battleID := p.nextBattleID()
	battleStart := &pb.CombatBattleStart{BattleId: battleID}
	characterUnit, err := p.newPlayerCharacterUnit(character)
	if err != nil {
		return err
	}
	petUnit, err := p.newPlayerPetUnit(character, battlePet)
	if err != nil {
		return err
	}
	enemyUnits, err := p.newEnemyUnits(battleID, enemyGroup, character.GetExp())
	if err != nil {
		return err
	}
	battleStart.UnitList = append(battleStart.UnitList, characterUnit, petUnit)
	battleStart.UnitList = append(battleStart.UnitList, enemyUnits...)

	p.combatState = &accountCombatState{
		battleID:        battleID,
		round:           1,
		gateway:         gateway,
		battleStart:     battleStart,
		playerCharacter: characterUnit,
		playerPet:       petUnit,
		enemyUnits:      enemyUnits,
		unitStates:      newCombatUnitRuntimeStates(battleStart.GetUnitList()),
		playerActions:   map[string]*pb.CombatActionIntent{},
	}
	p.sendClientPush(gateway, uint32(pb.MsgIDAccount_CombatBattleStartNotify_CMD), &pb.CombatBattleStartNotify{BattleStart: battleStart})
	p.beginCombatRound()
	return nil
}

func newCombatUnitRuntimeStates(units []*pb.CombatUnit) map[string]*combatUnitRuntimeState {
	states := make(map[string]*combatUnitRuntimeState, len(units))
	for _, unit := range units {
		if unit == nil || unit.GetKey() == nil {
			continue
		}
		maxHP := unit.GetAttribute().GetHp()
		states[combatUnitKeyMapKey(unit.GetKey())] = &combatUnitRuntimeState{
			unit:  unit,
			hp:    maxHP,
			maxHP: maxHP,
			alive: maxHP > 0,
		}
	}
	return states
}

func (p *Account) beginCombatRound() {
	p.clearRoundTimer()
	if p.combatState == nil {
		return
	}
	p.combatState.playerActions = map[string]*pb.CombatActionIntent{}
	p.combatState.resetRoundGuards()
	p.combatState.deadlineMs = time.Now().Add(combatRoundDuration).UnixMilli()
	p.startRoundTimer()
	p.sendCombatRoundPrepareNotify()
}

func (p *Account) startRoundTimer() {
	if p.combatState == nil || xtimer.GTimer == nil {
		return
	}
	p.roundTimerSeq++
	timerSeq := p.roundTimerSeq
	battleID := p.combatState.battleID
	round := p.combatState.round
	cb := xcontrol.NewCallBack(func(args ...any) error {
		if timerSeq != p.roundTimerSeq || p.combatState == nil || p.combatState.battleID != battleID || p.combatState.round != round {
			return nil
		}
		p.roundTimer = nil
		p.onCombatRoundTimeout()
		return nil
	})
	p.roundTimer = xtimer.GTimer.AddSecond(cb, time.Now().Add(combatRoundDuration).Unix(), p.actor)
}

func (p *Account) clearRoundTimer() {
	p.roundTimerSeq++
	if xtimer.GTimer != nil && p.roundTimer != nil {
		xtimer.GTimer.DelSecond(p.roundTimer)
	}
	p.roundTimer = nil
}

func (p *Account) sendCombatRoundPrepareNotify() {
	if p.combatState == nil {
		return
	}
	p.sendClientPush(p.combatState.gateway, uint32(pb.MsgIDAccount_CombatRoundPrepareNotify_CMD), &pb.CombatRoundPrepareNotify{
		BattleId:                 p.combatState.battleID,
		Round:                    p.combatState.round,
		ServerTimestampMs:        time.Now().UnixMilli(),
		RoundDeadlineTimestampMs: p.combatState.deadlineMs,
		UnitStateList:            p.combatState.unitStateSnapshots(),
		ActionOptionList:         p.combatActionOptions(),
		RequiredUnitKeyList:      p.combatState.requiredPlayerUnitKeys(),
		ReadyUnitKeyList:         p.combatState.readyPlayerUnitKeys(),
	})
}

func (p *Account) sendCombatRoundReadyNotify() {
	if p.combatState == nil {
		return
	}
	p.sendClientPush(p.combatState.gateway, uint32(pb.MsgIDAccount_CombatRoundReadyNotify_CMD), &pb.CombatRoundReadyNotify{
		BattleId:            p.combatState.battleID,
		Round:               p.combatState.round,
		RequiredUnitKeyList: p.combatState.requiredPlayerUnitKeys(),
		ReadyUnitKeyList:    p.combatState.readyPlayerUnitKeys(),
	})
}

func (p *Account) onCombatRoundTimeout() {
	if p.combatState == nil {
		return
	}
	if p.playerActionsReady() {
		p.completeCombatRound(p.collectedPlayerActions())
		return
	}
	p.completeCombatRound(p.timeoutPlayerActions())
}

func (p *Account) completeCombatRound(playerActions []*pb.CombatActionIntent) {
	if p.combatState == nil {
		return
	}
	p.clearRoundTimer()

	actions := append([]*pb.CombatActionIntent{}, playerActions...)
	actions = append(actions, p.enemyRoundActions()...)
	p.combatState.applyRoundGuards(actions)
	actions = p.sortedRoundIntents(actions)
	currentRound := p.combatState.round
	events, battleFinished, settlement := p.executeRoundIntents(actions)
	p.sendClientPush(p.combatState.gateway, uint32(pb.MsgIDAccount_CombatRoundResultNotify_CMD), &pb.CombatRoundResultNotify{
		BattleId:       p.combatState.battleID,
		Round:          currentRound,
		IntentList:     actions,
		BattleFinished: battleFinished,
		EventList:      events,
		Settlement:     settlement,
	})
	if battleFinished {
		p.finishCombatBattle()
		return
	}
	p.combatState.round++
	p.beginCombatRound()
}

func (p *Account) sortedRoundIntents(intents []*pb.CombatActionIntent) []*pb.CombatActionIntent {
	out := append([]*pb.CombatActionIntent{}, intents...)
	p.ensureRNG().Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	sort.SliceStable(out, func(i, j int) bool {
		return p.combatState.unitAgility(out[i].GetUnitKey()) > p.combatState.unitAgility(out[j].GetUnitKey())
	})
	return out
}

func (p *Account) executeRoundIntents(intents []*pb.CombatActionIntent) ([]*pb.CombatEvent, bool, *pb.CombatBattleSettlement) {
	events := make([]*pb.CombatEvent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		if finished, settlement := p.combatState.battleSettlementIfFinished(); finished {
			return events, true, settlement
		}
		sourceState := p.combatState.stateByKey(intent.GetUnitKey())
		if sourceState == nil || !sourceState.alive {
			continue
		}
		event := p.combatEventForIntent(intent, uint32(len(events)+1))
		if event != nil {
			events = append(events, event)
		}
		if finished, settlement := p.combatState.battleSettlementIfFinished(); finished {
			return events, true, settlement
		}
	}
	finished, settlement := p.combatState.battleSettlementIfFinished()
	return events, finished, settlement
}

func (p *Account) combatEventForIntent(intent *pb.CombatActionIntent, eventID uint32) *pb.CombatEvent {
	event := &pb.CombatEvent{
		EventId:           eventID,
		EventKind:         pb.CombatEventKind_CombatEventKind_Action,
		ActionType:        intent.GetActionType(),
		ActionId:          intent.GetActionId(),
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(intent.GetUnitKey())},
	}
	if combatIntentIsGuard(intent) {
		event.TargetUnitKeyList = append(event.TargetUnitKeyList, cloneCombatUnitKey(intent.GetUnitKey()))
		event.EffectList = append(event.EffectList, &pb.CombatEffect{
			EffectId:          1,
			EffectKind:        pb.CombatEffectKind_CombatEffectKind_Guard,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(intent.GetUnitKey())},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(intent.GetUnitKey())},
		})
		return event
	}
	if intent.GetActionType() == pb.CombatActionType_CombatActionType_Skill && intent.GetActionId() == petSkillStandby {
		return event
	}

	target := p.combatState.executionTarget(intent.GetUnitKey(), intent.GetTargetKey())
	if target == nil {
		event.EffectList = append(event.EffectList, &pb.CombatEffect{
			EffectId:          1,
			EffectKind:        pb.CombatEffectKind_CombatEffectKind_Miss,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(intent.GetUnitKey())},
		})
		return event
	}
	event.TargetUnitKeyList = append(event.TargetUnitKeyList, cloneCombatUnitKey(target.unit.GetKey()))
	for _, percent := range combatIntentDamagePercents(intent) {
		if !target.alive {
			break
		}
		p.appendDamageEffects(event, intent.GetUnitKey(), target, percent)
	}
	return event
}

func (p *Account) appendDamageEffects(event *pb.CombatEvent, sourceKey *pb.CombatUnitKey, target *combatUnitRuntimeState, percent uint64) {
	if event == nil || target == nil || !target.alive {
		return
	}
	source := p.combatState.stateByKey(sourceKey)
	if source == nil || source.unit == nil || source.unit.GetAttribute() == nil || target.unit == nil || target.unit.GetAttribute() == nil {
		return
	}
	base := combatBaseDamage(source.unit.GetAttribute().GetAttack(), target.unit.GetAttribute().GetDefense())
	damage := scaleCombatDamage(base, percent)
	if target.guard {
		damage = maxUint64(1, damage/2)
	}
	if damage > target.hp {
		damage = target.hp
	}
	if damage == 0 {
		return
	}
	target.hp -= damage
	effectID := uint32(len(event.GetEffectList()) + 1)
	event.EffectList = append(event.EffectList, &pb.CombatEffect{
		EffectId:          effectID,
		EffectKind:        pb.CombatEffectKind_CombatEffectKind_Damage,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(sourceKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(target.unit.GetKey())},
		UnitDeltaList: []*pb.CombatUnitStateDelta{{
			UnitKey: cloneCombatUnitKey(target.unit.GetKey()),
			AssetDeltaList: []*pb.CombatAssetDelta{{
				AssetType: pb.CombatAssetType_CombatAssetType_HP,
				Delta:     -int64(damage),
				After:     target.hp,
			}},
		}},
	})
	if target.hp == 0 && target.alive {
		target.alive = false
		event.EffectList = append(event.EffectList, &pb.CombatEffect{
			EffectId:          uint32(len(event.GetEffectList()) + 1),
			EffectKind:        pb.CombatEffectKind_CombatEffectKind_UnitAlive,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(sourceKey)},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(target.unit.GetKey())},
			UnitDeltaList: []*pb.CombatUnitStateDelta{{
				UnitKey:      cloneCombatUnitKey(target.unit.GetKey()),
				AliveChanged: true,
				Alive:        false,
			}},
		})
	}
}

func combatBaseDamage(attack uint64, defense uint64) uint64 {
	reduced := defense / 2
	if attack <= reduced {
		return 1
	}
	return attack - reduced
}

func scaleCombatDamage(base uint64, percent uint64) uint64 {
	damage := base * percent / 100
	return maxUint64(1, damage)
}

func combatIntentDamagePercents(intent *pb.CombatActionIntent) []uint64 {
	if intent == nil {
		return nil
	}
	if intent.GetActionType() == pb.CombatActionType_CombatActionType_Attack {
		return []uint64{100}
	}
	if intent.GetActionType() != pb.CombatActionType_CombatActionType_Skill {
		return nil
	}
	switch intent.GetActionId() {
	case petSkillStandby, petSkillDefense:
		return nil
	case petSkillDoubleAttack:
		return []uint64{65, 65}
	case petSkillTripleAttack:
		return []uint64{50, 50, 50}
	default:
		return []uint64{100}
	}
}

func combatIntentIsGuard(intent *pb.CombatActionIntent) bool {
	if intent == nil {
		return false
	}
	return intent.GetActionType() == pb.CombatActionType_CombatActionType_Defense ||
		(intent.GetActionType() == pb.CombatActionType_CombatActionType_Skill && intent.GetActionId() == petSkillDefense)
}

func (p *Account) playerActionsReady() bool {
	if p.combatState == nil {
		return false
	}
	for _, key := range p.combatState.requiredPlayerUnitKeys() {
		if _, ok := p.combatState.playerActions[combatUnitKeyMapKey(key)]; !ok {
			return false
		}
	}
	return true
}

func (p *Account) collectedPlayerActions() []*pb.CombatActionIntent {
	if p.combatState == nil {
		return nil
	}
	actions := make([]*pb.CombatActionIntent, 0, 2)
	for _, key := range p.combatState.requiredPlayerUnitKeys() {
		if action := p.combatState.playerActions[combatUnitKeyMapKey(key)]; action != nil {
			actions = append(actions, action)
		}
	}
	return actions
}

func (p *Account) timeoutPlayerActions() []*pb.CombatActionIntent {
	if p.combatState == nil {
		return nil
	}
	actions := make([]*pb.CombatActionIntent, 0, 2)
	characterKey := combatUnitKeyMapKey(p.combatState.playerCharacter.GetKey())
	petKey := combatUnitKeyMapKey(p.combatState.playerPet.GetKey())
	if p.combatState.isAlive(p.combatState.playerCharacter.GetKey()) {
		if action := p.combatState.playerActions[characterKey]; action != nil {
			actions = append(actions, action)
		} else {
			actions = append(actions, p.defaultCharacterAction())
		}
	}
	if p.combatState.isAlive(p.combatState.playerPet.GetKey()) {
		if action := p.combatState.playerActions[petKey]; action != nil {
			actions = append(actions, action)
		} else {
			actions = append(actions, p.defaultPetAction())
		}
	}
	return actions
}

func (p *Account) finishCombatBattle() {
	if p.combatState == nil {
		return
	}
	gateway := p.combatState.gateway
	p.clearRoundTimer()
	p.combatState = nil
	if p.autoEncounterEnabled {
		p.restartAutoEncounterTimer(gateway)
	}
}

func (p *Account) clearCombatRuntime() {
	p.clearAutoEncounterTimer()
	p.clearRoundTimer()
	p.combatState = nil
	p.autoEncounterEnabled = false
}

func (p *Account) sendClientPush(gateway *Gateway, messageID uint32, message proto.Message) {
	if gateway == nil {
		return
	}
	body, err := proto.Marshal(message)
	if err != nil {
		xlog.GLog.Errorf("marshal client push failed aid:%d messageID:%d err:%v", p.aid, messageID, err)
		return
	}
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: 0,
				ResultId:  xerror.Success.Code(),
				Key:       p.aid,
				Body:      body,
			},
		},
	})
}

func (p *Account) currentBattleCharacterAndPet() (*pb.CharacterRecord, *pb.PetRecord, error) {
	if p.accountRecord == nil || p.accountRecord.GetAccountRecordCreateTimestampMs() == 0 {
		return nil, nil, fmt.Errorf("account record is not initialized")
	}
	if p.activeCharacterUUID == 0 || !p.isCharacterOnline(p.activeCharacterUUID) {
		return nil, nil, fmt.Errorf("active online character not found")
	}
	character := p.findCharacterRecord(p.activeCharacterUUID)
	if character == nil {
		return nil, nil, fmt.Errorf("active character record not found: %d", p.activeCharacterUUID)
	}
	for _, pet := range character.GetPetRecordList() {
		if pet != nil && pet.GetCarryStatus() == pb.PetCarryStatus_PetCarryStatus_Battle {
			return character, pet, nil
		}
	}
	return nil, nil, fmt.Errorf("battle pet not found character:%d", character.GetUuid())
}

func (p *Account) newPlayerCharacterUnit(character *pb.CharacterRecord) (*pb.CombatUnit, error) {
	if character == nil || character.GetUuid() == 0 || character.GetAssetId() == 0 {
		return nil, fmt.Errorf("character record invalid")
	}
	attribute := character.GetAttribute()
	if attribute == nil {
		return nil, fmt.Errorf("character attribute missing character:%d", character.GetUuid())
	}
	vitality := uint64(attribute.GetVitality())
	strength := uint64(attribute.GetStrength())
	toughness := uint64(attribute.GetToughness())
	dexterity := uint64(attribute.GetDexterity())
	hp := characterRuntimeHP(vitality, strength, toughness, dexterity)
	return &pb.CombatUnit{
		Camp:        pb.CombatCamp_CombatCamp_Initiator,
		Position:    initiatorCharacterPosition,
		Key:         &pb.CombatUnitKey{Aid: p.aid, CharacterUuid: character.GetUuid()},
		CharacterId: uint32(character.GetAssetId()),
		Attribute: &pb.CombatUnitAttribute{
			Exp:     character.GetExp(),
			Hp:      hp,
			Attack:  maxUint64(1, strength+toughness/10+vitality/10+dexterity/20),
			Defense: maxUint64(1, toughness+strength/10+vitality/10+dexterity/20),
			Agility: maxUint64(1, dexterity),
		},
	}, nil
}

func characterRuntimeHP(vitality uint64, strength uint64, toughness uint64, dexterity uint64) uint64 {
	return vitality*4 + strength + toughness + dexterity
}

func (p *Account) newPlayerPetUnit(character *pb.CharacterRecord, pet *pb.PetRecord) (*pb.CombatUnit, error) {
	if character == nil || pet == nil || pet.GetUuid() == 0 {
		return nil, fmt.Errorf("pet record invalid")
	}
	records := pet.GetAssetRecordBaseMap()
	petID := uint32(records[uint32(pb.AssetIDRecord_AssetIDRecord_AssetID)])
	if petID == 0 {
		return nil, fmt.Errorf("pet asset id is empty pet:%d", pet.GetUuid())
	}
	attr := petRuntimeAttributeFromPetRecord(pet)
	if attr == nil {
		return nil, fmt.Errorf("pet runtime attribute missing pet:%d", pet.GetUuid())
	}
	return &pb.CombatUnit{
		Camp:        pb.CombatCamp_CombatCamp_Initiator,
		Position:    initiatorPetPosition,
		Key:         &pb.CombatUnitKey{Aid: p.aid, CharacterUuid: character.GetUuid(), PetUuid: pet.GetUuid()},
		CharacterId: uint32(character.GetAssetId()),
		PetId:       petID,
		Attribute: &pb.CombatUnitAttribute{
			Exp:     pet.GetExp(),
			Hp:      attr.hp,
			Attack:  attr.attack,
			Defense: attr.defense,
			Agility: attr.agility,
			Loyalty: pet.GetLoyalty(),
		},
	}, nil
}

func (p *Account) newEnemyUnits(battleID uint64, enemyGroup *gameconfig.EnemyGroupEntry, characterExp uint64) ([]*pb.CombatUnit, error) {
	selected, err := p.selectEnemies(enemyGroup)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.CombatUnit, 0, len(selected))
	for index, enemy := range selected {
		level, err := p.enemyLevel(enemyGroup, enemy, characterExp)
		if err != nil {
			return nil, err
		}
		attr, err := createPetRuntimeAttribute(uint32(enemy.ID), level, unspecifiedPetGrade, p.ensureRNG())
		if err != nil {
			return nil, err
		}
		minExp, err := GGameConfig.Exp.GetLevelMinExp(level)
		if err != nil {
			return nil, err
		}
		out = append(out, &pb.CombatUnit{
			Camp:     pb.CombatCamp_CombatCamp_Defender,
			Position: uint32(index),
			Key:      &pb.CombatUnitKey{PetUuid: battleID*100 + uint64(index) + 1},
			PetId:    uint32(enemy.ID),
			Attribute: &pb.CombatUnitAttribute{
				Exp:     uint64(minExp),
				Hp:      attr.hp,
				Attack:  attr.attack,
				Defense: attr.defense,
				Agility: attr.agility,
			},
		})
	}
	return out, nil
}

func (p *Account) randomSceneEnemyGroup(sceneID uint32) (*gameconfig.EnemyGroupEntry, error) {
	if GGameConfig == nil || GGameConfig.Scene == nil || GGameConfig.Enemy == nil {
		return nil, fmt.Errorf("scene or enemy config is not loaded")
	}
	scene := GGameConfig.Scene.GetByID(int(sceneID))
	if scene == nil {
		return nil, fmt.Errorf("scene not found: %d", sceneID)
	}
	entry, ok := p.chooseWeightedSceneEnemyGroup(scene.EnemyGroups)
	if !ok {
		return nil, fmt.Errorf("scene has no selectable enemy group: %d", sceneID)
	}
	group := GGameConfig.Enemy.GetByID(entry.ID)
	if group == nil {
		return nil, fmt.Errorf("scene enemy group not found: scene:%d group:%d", sceneID, entry.ID)
	}
	return group, nil
}

func (p *Account) chooseWeightedSceneEnemyGroup(weighted []gameconfig.SceneEnemyGroupEntry) (gameconfig.SceneEnemyGroupEntry, bool) {
	total := 0
	for _, group := range weighted {
		if group.Weight > 0 {
			total += group.Weight
		}
	}
	if total <= 0 {
		return gameconfig.SceneEnemyGroupEntry{}, false
	}
	roll := p.ensureRNG().Intn(total) + 1
	current := 0
	for _, group := range weighted {
		if group.Weight <= 0 {
			continue
		}
		current += group.Weight
		if roll <= current {
			return group, true
		}
	}
	return gameconfig.SceneEnemyGroupEntry{}, false
}

func (p *Account) selectEnemies(group *gameconfig.EnemyGroupEntry) ([]gameconfig.EnemyEntry, error) {
	if group == nil || len(group.Enemies) == 0 {
		return nil, fmt.Errorf("enemy group is empty")
	}
	if group.IsBoss {
		count := minInt(len(group.Enemies), int(pb.CombatCampPosition_CombatCampPosition_Count))
		return append([]gameconfig.EnemyEntry(nil), group.Enemies[:count]...), nil
	}
	targetCount := p.ensureRNG().Intn(group.CountRange.Max-group.CountRange.Min+1) + group.CountRange.Min
	targetCount = clampInt(targetCount, 1, int(pb.CombatCampPosition_CombatCampPosition_Count))

	required := make([]gameconfig.EnemyEntry, 0, len(group.Enemies))
	weighted := make([]gameconfig.EnemyEntry, 0, len(group.Enemies))
	for _, enemy := range group.Enemies {
		if enemy.Weight <= 0 {
			required = append(required, enemy)
		} else {
			weighted = append(weighted, enemy)
		}
	}
	selected := make([]gameconfig.EnemyEntry, 0, targetCount)
	for _, enemy := range required {
		if len(selected) >= int(pb.CombatCampPosition_CombatCampPosition_Count) {
			break
		}
		selected = append(selected, enemy)
	}
	if targetCount < len(selected) {
		targetCount = len(selected)
	}
	for len(selected) < targetCount {
		enemy, ok := p.chooseWeightedEnemy(weighted)
		if !ok {
			if len(required) == 0 {
				return nil, fmt.Errorf("enemy group has no selectable enemy: %d", group.ID)
			}
			enemy = required[len(selected)%len(required)]
		}
		selected = append(selected, enemy)
	}
	return selected, nil
}

func (p *Account) chooseWeightedEnemy(weighted []gameconfig.EnemyEntry) (gameconfig.EnemyEntry, bool) {
	total := 0
	for _, enemy := range weighted {
		if enemy.Weight > 0 {
			total += enemy.Weight
		}
	}
	if total <= 0 {
		return gameconfig.EnemyEntry{}, false
	}
	roll := p.ensureRNG().Intn(total) + 1
	current := 0
	for _, enemy := range weighted {
		if enemy.Weight <= 0 {
			continue
		}
		current += enemy.Weight
		if roll <= current {
			return enemy, true
		}
	}
	return gameconfig.EnemyEntry{}, false
}

func (p *Account) enemyLevel(group *gameconfig.EnemyGroupEntry, enemy gameconfig.EnemyEntry, characterExp uint64) (int, error) {
	if enemy.Level > 0 {
		return enemy.Level, nil
	}
	if group.RoleLevelOffset.Min != 0 || group.RoleLevelOffset.Max != 0 {
		if GGameConfig == nil || GGameConfig.Exp == nil {
			return 0, fmt.Errorf("exp config is not loaded")
		}
		characterLevel, err := GGameConfig.Exp.GetLevel(int(characterExp))
		if err != nil {
			return 0, err
		}
		offset := p.ensureRNG().Intn(group.RoleLevelOffset.Max-group.RoleLevelOffset.Min+1) + group.RoleLevelOffset.Min
		return clampInt(characterLevel+offset, int(pb.LevelRange_LevelRange_Min), int(pb.LevelRange_LevelRange_Max)), nil
	}
	return p.ensureRNG().Intn(group.LevelRange.Max-group.LevelRange.Min+1) + group.LevelRange.Min, nil
}

func (p *Account) playerUnitAction(req *pb.CombatRoundActionReq) (*pb.CombatActionIntent, error) {
	if combatUnitKeyEqual(req.GetUnitKey(), p.combatState.playerCharacter.GetKey()) {
		return p.playerCharacterAction(req)
	}
	if combatUnitKeyEqual(req.GetUnitKey(), p.combatState.playerPet.GetKey()) {
		return p.playerPetAction(req)
	}
	return nil, fmt.Errorf("unit is not controlled by player")
}

func (p *Account) playerCharacterAction(req *pb.CombatRoundActionReq) (*pb.CombatActionIntent, error) {
	actionType := req.GetActionType()
	actionID := req.GetActionId()
	if !p.combatState.isAlive(p.combatState.playerCharacter.GetKey()) {
		return nil, fmt.Errorf("character is not alive")
	}
	if actionType == pb.CombatActionType_CombatActionType_Attack && actionID != uint32(pb.CharacterAction_CharacterAction_Attack) {
		return nil, fmt.Errorf("character attack action id invalid: %d", actionID)
	}
	if actionType == pb.CombatActionType_CombatActionType_Defense && actionID != uint32(pb.CharacterAction_CharacterAction_Defense) {
		return nil, fmt.Errorf("character defense action id invalid: %d", actionID)
	}
	if actionType != pb.CombatActionType_CombatActionType_Attack && actionType != pb.CombatActionType_CombatActionType_Defense {
		return nil, fmt.Errorf("unsupported character action type: %s", actionType)
	}

	target := cloneCombatUnitKey(p.combatState.playerCharacter.GetKey())
	if actionType == pb.CombatActionType_CombatActionType_Attack {
		var err error
		target, err = p.combatState.validOpponentTarget(req.GetTargetKey(), p.combatState.playerCharacter.GetKey())
		if err != nil {
			return nil, err
		}
	}
	return &pb.CombatActionIntent{
		UnitKey:      cloneCombatUnitKey(p.combatState.playerCharacter.GetKey()),
		ActionType:   actionType,
		ActionId:     actionID,
		TargetKey:    target,
		IntentSource: pb.CombatIntentSource_CombatIntentSource_Player,
	}, nil
}

func (p *Account) playerPetAction(req *pb.CombatRoundActionReq) (*pb.CombatActionIntent, error) {
	if !p.combatState.isAlive(p.combatState.playerPet.GetKey()) {
		return nil, fmt.Errorf("pet is not alive")
	}
	if req.GetActionType() != pb.CombatActionType_CombatActionType_Skill {
		return nil, fmt.Errorf("unsupported pet action type: %s", req.GetActionType())
	}
	skillID := req.GetActionId()
	if !p.petHasSkill(p.combatState.playerPet.GetPetId(), skillID) {
		return nil, fmt.Errorf("pet skill invalid pet:%d skill:%d", p.combatState.playerPet.GetPetId(), skillID)
	}
	target := cloneCombatUnitKey(p.combatState.playerPet.GetKey())
	if skillID != petSkillStandby && skillID != petSkillDefense {
		var err error
		target, err = p.combatState.validOpponentTarget(req.GetTargetKey(), p.combatState.playerPet.GetKey())
		if err != nil {
			return nil, err
		}
	}
	return &pb.CombatActionIntent{
		UnitKey:      cloneCombatUnitKey(p.combatState.playerPet.GetKey()),
		ActionType:   pb.CombatActionType_CombatActionType_Skill,
		ActionId:     skillID,
		TargetKey:    target,
		IntentSource: pb.CombatIntentSource_CombatIntentSource_Player,
	}, nil
}

func (p *Account) defaultCharacterAction() *pb.CombatActionIntent {
	return &pb.CombatActionIntent{
		UnitKey:      cloneCombatUnitKey(p.combatState.playerCharacter.GetKey()),
		ActionType:   pb.CombatActionType_CombatActionType_Defense,
		ActionId:     uint32(pb.CharacterAction_CharacterAction_Defense),
		TargetKey:    cloneCombatUnitKey(p.combatState.playerCharacter.GetKey()),
		IntentSource: pb.CombatIntentSource_CombatIntentSource_TimeoutDefault,
	}
}

func (p *Account) defaultPetAction() *pb.CombatActionIntent {
	skillID := p.randomPetSkill(p.combatState.playerPet.GetPetId())
	target := cloneCombatUnitKey(p.combatState.playerPet.GetKey())
	if skillID != petSkillStandby && skillID != petSkillDefense {
		target = p.combatState.firstAliveOpponentKey(p.combatState.playerPet.GetKey())
	}
	return &pb.CombatActionIntent{
		UnitKey:      cloneCombatUnitKey(p.combatState.playerPet.GetKey()),
		ActionType:   pb.CombatActionType_CombatActionType_Skill,
		ActionId:     skillID,
		TargetKey:    target,
		IntentSource: pb.CombatIntentSource_CombatIntentSource_TimeoutDefault,
	}
}

func (p *Account) enemyRoundActions() []*pb.CombatActionIntent {
	if p.combatState == nil {
		return nil
	}
	actions := make([]*pb.CombatActionIntent, 0, len(p.combatState.enemyUnits))
	for _, unit := range p.combatState.enemyUnits {
		if !p.combatState.isAlive(unit.GetKey()) {
			continue
		}
		skillID := p.randomPetSkill(unit.GetPetId())
		target := cloneCombatUnitKey(unit.GetKey())
		if skillID != petSkillStandby && skillID != petSkillDefense {
			target = p.combatState.firstAliveOpponentKey(unit.GetKey())
		}
		actions = append(actions, &pb.CombatActionIntent{
			UnitKey:      cloneCombatUnitKey(unit.GetKey()),
			ActionType:   pb.CombatActionType_CombatActionType_Skill,
			ActionId:     skillID,
			TargetKey:    target,
			IntentSource: pb.CombatIntentSource_CombatIntentSource_EnemyAI,
		})
	}
	return actions
}

func (p *Account) combatActionOptions() []*pb.CombatActionOption {
	if p.combatState == nil {
		return nil
	}
	enemyTargets := p.combatState.aliveOpponentKeys(p.combatState.playerCharacter.GetKey())
	options := make([]*pb.CombatActionOption, 0)
	if p.combatState.isAlive(p.combatState.playerCharacter.GetKey()) {
		options = append(options, &pb.CombatActionOption{
			UnitKey:       cloneCombatUnitKey(p.combatState.playerCharacter.GetKey()),
			ActionType:    pb.CombatActionType_CombatActionType_Attack,
			ActionId:      uint32(pb.CharacterAction_CharacterAction_Attack),
			TargetKeyList: cloneCombatUnitKeyList(enemyTargets),
		})
		options = append(options, &pb.CombatActionOption{
			UnitKey:    cloneCombatUnitKey(p.combatState.playerCharacter.GetKey()),
			ActionType: pb.CombatActionType_CombatActionType_Defense,
			ActionId:   uint32(pb.CharacterAction_CharacterAction_Defense),
		})
	}
	if p.combatState.isAlive(p.combatState.playerPet.GetKey()) {
		pet := GGameConfig.Pet.GetByID(int(p.combatState.playerPet.GetPetId()))
		if pet != nil {
			for _, skillID := range pet.SkillSlots {
				if skillID == 0 {
					continue
				}
				option := &pb.CombatActionOption{
					UnitKey:    cloneCombatUnitKey(p.combatState.playerPet.GetKey()),
					ActionType: pb.CombatActionType_CombatActionType_Skill,
					ActionId:   uint32(skillID),
				}
				if skillID != petSkillStandby && skillID != petSkillDefense {
					option.TargetKeyList = cloneCombatUnitKeyList(enemyTargets)
				}
				options = append(options, option)
			}
		}
	}
	return options
}

func (p *Account) petHasSkill(petID uint32, skillID uint32) bool {
	if skillID == 0 || GGameConfig == nil || GGameConfig.Pet == nil {
		return false
	}
	pet := GGameConfig.Pet.GetByID(int(petID))
	if pet == nil {
		return false
	}
	for _, candidate := range pet.SkillSlots {
		if candidate != 0 && uint32(candidate) == skillID {
			return true
		}
	}
	return false
}

func (p *Account) randomPetSkill(petID uint32) uint32 {
	if GGameConfig == nil || GGameConfig.Pet == nil {
		return 0
	}
	pet := GGameConfig.Pet.GetByID(int(petID))
	if pet == nil {
		return 0
	}
	skills := make([]int, 0, len(pet.SkillSlots))
	for _, skillID := range pet.SkillSlots {
		if skillID != 0 {
			skills = append(skills, skillID)
		}
	}
	if len(skills) == 0 {
		return 0
	}
	return uint32(skills[p.ensureRNG().Intn(len(skills))])
}

func (p *Account) nextBattleID() uint64 {
	p.lastBattleID++
	return uint64(time.Now().UnixMilli())*100000 + p.aid%100000 + p.lastBattleID
}

func (p *Account) ensureRNG() *rand.Rand {
	if p.rng == nil {
		p.rng = rand.New(rand.NewSource(time.Now().UnixNano() + int64(p.aid)))
	}
	return p.rng
}

func (s *accountCombatState) resetRoundGuards() {
	if s == nil {
		return
	}
	for _, state := range s.unitStates {
		state.guard = false
	}
}

func (s *accountCombatState) applyRoundGuards(intents []*pb.CombatActionIntent) {
	s.resetRoundGuards()
	for _, intent := range intents {
		if !combatIntentIsGuard(intent) {
			continue
		}
		state := s.stateByKey(intent.GetUnitKey())
		if state != nil && state.alive {
			state.guard = true
		}
	}
}

func (s *accountCombatState) unitStateSnapshots() []*pb.CombatUnitState {
	if s == nil {
		return nil
	}
	out := make([]*pb.CombatUnitState, 0, len(s.battleStart.GetUnitList()))
	for _, unit := range s.battleStart.GetUnitList() {
		state := s.stateByKey(unit.GetKey())
		if state == nil {
			continue
		}
		out = append(out, &pb.CombatUnitState{
			UnitKey:         cloneCombatUnitKey(unit.GetKey()),
			Hp:              state.hp,
			MaxHp:           state.maxHP,
			Alive:           state.alive,
			StatusStateList: cloneCombatStatusStateList(state.statusStates),
		})
	}
	return out
}

func (s *accountCombatState) requiredPlayerUnitKeys() []*pb.CombatUnitKey {
	if s == nil {
		return nil
	}
	keys := make([]*pb.CombatUnitKey, 0, 2)
	if s.isAlive(s.playerCharacter.GetKey()) {
		keys = append(keys, cloneCombatUnitKey(s.playerCharacter.GetKey()))
	}
	if s.isAlive(s.playerPet.GetKey()) {
		keys = append(keys, cloneCombatUnitKey(s.playerPet.GetKey()))
	}
	return keys
}

func (s *accountCombatState) readyPlayerUnitKeys() []*pb.CombatUnitKey {
	if s == nil {
		return nil
	}
	keys := make([]*pb.CombatUnitKey, 0, 2)
	for _, key := range s.requiredPlayerUnitKeys() {
		if _, ok := s.playerActions[combatUnitKeyMapKey(key)]; ok {
			keys = append(keys, cloneCombatUnitKey(key))
		}
	}
	return keys
}

func (s *accountCombatState) stateByKey(key *pb.CombatUnitKey) *combatUnitRuntimeState {
	if s == nil || key == nil {
		return nil
	}
	return s.unitStates[combatUnitKeyMapKey(key)]
}

func (s *accountCombatState) isAlive(key *pb.CombatUnitKey) bool {
	state := s.stateByKey(key)
	return state != nil && state.alive
}

func (s *accountCombatState) unitAgility(key *pb.CombatUnitKey) uint64 {
	state := s.stateByKey(key)
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return 0
	}
	return state.unit.GetAttribute().GetAgility()
}

func (s *accountCombatState) unitCamp(key *pb.CombatUnitKey) (pb.CombatCamp, bool) {
	state := s.stateByKey(key)
	if state == nil || state.unit == nil {
		return pb.CombatCamp_CombatCamp_Initiator, false
	}
	return state.unit.GetCamp(), true
}

func (s *accountCombatState) validOpponentTarget(requested *pb.CombatUnitKey, source *pb.CombatUnitKey) (*pb.CombatUnitKey, error) {
	if requested == nil || combatUnitKeyEmpty(requested) {
		return nil, fmt.Errorf("target is required")
	}
	if !s.isAlive(requested) {
		return nil, fmt.Errorf("target is not alive")
	}
	sourceCamp, ok := s.unitCamp(source)
	if !ok {
		return nil, fmt.Errorf("source unit not found")
	}
	targetCamp, ok := s.unitCamp(requested)
	if !ok {
		return nil, fmt.Errorf("target unit not found")
	}
	if sourceCamp == targetCamp {
		return nil, fmt.Errorf("target must be opponent")
	}
	return cloneCombatUnitKey(requested), nil
}

func (s *accountCombatState) executionTarget(source *pb.CombatUnitKey, requested *pb.CombatUnitKey) *combatUnitRuntimeState {
	if requested != nil && !combatUnitKeyEmpty(requested) {
		if target, err := s.validOpponentTarget(requested, source); err == nil {
			return s.stateByKey(target)
		}
	}
	return s.stateByKey(s.firstAliveOpponentKey(source))
}

func (s *accountCombatState) firstAliveOpponentKey(source *pb.CombatUnitKey) *pb.CombatUnitKey {
	keys := s.aliveOpponentKeys(source)
	if len(keys) == 0 {
		return nil
	}
	return keys[0]
}

func (s *accountCombatState) aliveOpponentKeys(source *pb.CombatUnitKey) []*pb.CombatUnitKey {
	if s == nil || source == nil {
		return nil
	}
	sourceCamp, ok := s.unitCamp(source)
	if !ok {
		return nil
	}
	keys := make([]*pb.CombatUnitKey, 0)
	for _, unit := range s.battleStart.GetUnitList() {
		if unit.GetCamp() == sourceCamp || !s.isAlive(unit.GetKey()) {
			continue
		}
		keys = append(keys, cloneCombatUnitKey(unit.GetKey()))
	}
	return keys
}

func (s *accountCombatState) battleSettlementIfFinished() (bool, *pb.CombatBattleSettlement) {
	if s == nil {
		return false, nil
	}
	initiatorAlive := s.campBattleAlive(pb.CombatCamp_CombatCamp_Initiator)
	defenderAlive := s.campBattleAlive(pb.CombatCamp_CombatCamp_Defender)
	if initiatorAlive && defenderAlive {
		return false, nil
	}
	settlement := &pb.CombatBattleSettlement{}
	switch {
	case initiatorAlive && !defenderAlive:
		settlement.EndReason = pb.CombatBattleEndReason_CombatBattleEndReason_InitiatorWin
		settlement.WinnerUnitKeyList = s.unitKeysByCamp(pb.CombatCamp_CombatCamp_Initiator)
		settlement.LoserUnitKeyList = s.unitKeysByCamp(pb.CombatCamp_CombatCamp_Defender)
	case defenderAlive && !initiatorAlive:
		settlement.EndReason = pb.CombatBattleEndReason_CombatBattleEndReason_DefenderWin
		settlement.WinnerUnitKeyList = s.unitKeysByCamp(pb.CombatCamp_CombatCamp_Defender)
		settlement.LoserUnitKeyList = s.unitKeysByCamp(pb.CombatCamp_CombatCamp_Initiator)
	default:
		settlement.EndReason = pb.CombatBattleEndReason_CombatBattleEndReason_Draw
	}
	return true, settlement
}

func (s *accountCombatState) campBattleAlive(camp pb.CombatCamp) bool {
	if s.campHasCharacterUnits(camp) {
		return s.campCharacterAlive(camp)
	}
	for _, unit := range s.battleStart.GetUnitList() {
		if unit.GetCamp() == camp && s.isAlive(unit.GetKey()) {
			return true
		}
	}
	return false
}

func (s *accountCombatState) campHasCharacterUnits(camp pb.CombatCamp) bool {
	for _, unit := range s.battleStart.GetUnitList() {
		if unit.GetCamp() == camp && combatUnitIsCharacter(unit) {
			return true
		}
	}
	return false
}

func (s *accountCombatState) campCharacterAlive(camp pb.CombatCamp) bool {
	for _, unit := range s.battleStart.GetUnitList() {
		if unit.GetCamp() == camp && combatUnitIsCharacter(unit) && s.isAlive(unit.GetKey()) {
			return true
		}
	}
	return false
}

func combatUnitIsCharacter(unit *pb.CombatUnit) bool {
	return unit != nil && unit.GetCharacterId() != 0 && unit.GetPetId() == 0
}

func (s *accountCombatState) unitKeysByCamp(camp pb.CombatCamp) []*pb.CombatUnitKey {
	keys := make([]*pb.CombatUnitKey, 0)
	for _, unit := range s.battleStart.GetUnitList() {
		if unit.GetCamp() == camp {
			keys = append(keys, cloneCombatUnitKey(unit.GetKey()))
		}
	}
	return keys
}

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

func cloneCombatUnitKeyList(keys []*pb.CombatUnitKey) []*pb.CombatUnitKey {
	out := make([]*pb.CombatUnitKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, cloneCombatUnitKey(key))
	}
	return out
}

func cloneCombatStatusStateList(states []*pb.CombatStatusState) []*pb.CombatStatusState {
	out := make([]*pb.CombatStatusState, 0, len(states))
	for _, state := range states {
		if state == nil {
			continue
		}
		out = append(out, &pb.CombatStatusState{
			StatusId:    state.GetStatusId(),
			Stack:       state.GetStack(),
			ExpireRound: state.GetExpireRound(),
		})
	}
	return out
}

func combatUnitKeyEqual(a *pb.CombatUnitKey, b *pb.CombatUnitKey) bool {
	if a == nil || b == nil {
		return false
	}
	return a.GetAid() == b.GetAid() &&
		a.GetCharacterUuid() == b.GetCharacterUuid() &&
		a.GetPetUuid() == b.GetPetUuid()
}

func combatUnitKeyEmpty(key *pb.CombatUnitKey) bool {
	return key.GetAid() == 0 && key.GetCharacterUuid() == 0 && key.GetPetUuid() == 0
}

func combatUnitKeyMapKey(key *pb.CombatUnitKey) string {
	if key == nil {
		return "0:0:0"
	}
	return fmt.Sprintf("%d:%d:%d", key.GetAid(), key.GetCharacterUuid(), key.GetPetUuid())
}

func maxUint64(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
