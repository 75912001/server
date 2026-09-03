package main

import (
	"context"
	"fmt"
	"time"

	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
)

const (
	OnlineAccountActorCmdBind         xactor.CMD = 101
	OnlineAccountActorCmdUnbind       xactor.CMD = 102
	OnlineAccountActorCmdClientPacket xactor.CMD = 103
	OnlineAccountActorCmdStop         xactor.CMD = 104
	OnlineAccountActorCmdRoomFinished xactor.CMD = 105
	OnlineAccountActorCmdTeamScene    xactor.CMD = 106
	OnlineAccountActorCmdTryBindRoom  xactor.CMD = 107
	OnlineAccountActorCmdRoomLeft     xactor.CMD = 108
	OnlineAccountActorCmdCapturePet   xactor.CMD = 109
)

func (p *Account) PostBind(req *pb.OnlineBindAccountReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindAccountRes, error) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdBind, req, accountRecord))
	if err != nil {
		return nil, err
	}
	res, _ := resp.(*pb.OnlineBindAccountRes)
	return res, nil
}

func (p *Account) PostUnbind(gatewayKey string, accountSession string) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdUnbind, gatewayKey, accountSession))
	if err != nil {
		xlog.GLog.Errorf("account unbind sync failed aid=%d err=%v", p.aid, err)
		return
	}
	stopped, _ := resp.(bool)
	if stopped {
		p.actor.SendMsg(xactor.NewMsg(context.Background(), xactor.SystemReservedCommand_Stop))
	}
}

func (p *Account) PostClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	p.actor.SendMsg(xactor.NewMsg(context.Background(), OnlineAccountActorCmdClientPacket, gateway, pkt))
}

// PostCombatRoomFinishedSync 同步请求 Account actor 清除匹配的 CombatRoom actor 指针并投递最终战报.
func (p *Account) PostCombatRoomFinishedSync(input combatRoomFinishInput) error {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdRoomFinished, input))
	if err != nil {
		return err
	}
	if finishErr, ok := resp.(error); ok {
		return finishErr
	}
	return nil
}

// PostTryBindCombatRoomSync 请求目标 Account actor 原子生成入场快照并绑定 CombatRoom.
func (p *Account) PostTryBindCombatRoomSync(input combatRoomTryBindInput) (combatRoomTryBindResult, error) {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdTryBindRoom, input))
	if err != nil {
		return combatRoomTryBindResult{}, err
	}
	result, ok := resp.(combatRoomTryBindResult)
	if !ok {
		return combatRoomTryBindResult{}, fmt.Errorf("combat room bind response invalid")
	}
	return result, nil
}

// PostCombatRoomParticipantLeftSync 同步清除单个参与者的匹配房间指针并投递其最后一份回合战报.
func (p *Account) PostCombatRoomParticipantLeftSync(input combatRoomParticipantLeaveInput) error {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdRoomLeft, input))
	if err != nil {
		return err
	}
	if leaveErr, ok := resp.(error); ok {
		return leaveErr
	}
	return nil
}

func (p *Account) behavior(messages ...any) (xactor.Behavior, any, error) {
	var resp any
	for _, raw := range messages {
		if event, ok := raw.(*xcontrol.Event); ok {
			if event.ISwitch.IsOn() {
				if errTmp := event.ICallBack.Execute(); errTmp != nil {
					xlog.GLog.Warnf("account event callback failed aid=%d err=%v", p.aid, errTmp)
				}
			}
			continue
		}
		msg, ok := raw.(*xactor.Msg)
		if !ok {
			continue
		}
		switch msg.Cmd {
		case OnlineAccountActorCmdBind:
			if len(msg.Args) < 2 {
				continue
			}
			req, ok := msg.Args[0].(*pb.OnlineBindAccountReq)
			if !ok {
				continue
			}
			accountRecord, ok := msg.Args[1].(*pb.AccountRecord)
			if !ok {
				continue
			}
			characterManager := newCharacterMgr(p, accountRecord)

			p.characterManager.clearRuntime()
			p.gatewayKey = req.GetGatewayKey()
			p.accountSession = req.GetAccountSession()
			p.account = req.GetAccount()
			p.clientIP = req.GetClientIp()
			p.accountRecord = accountRecord
			p.characterManager = characterManager
			GAccountMgr.accounts.Add(p.aid, p)
			resp = &pb.OnlineBindAccountRes{}
		case OnlineAccountActorCmdUnbind:
			gatewayKey, ok := msg.Args[0].(string)
			if !ok {
				continue
			}
			accountSession, ok := msg.Args[1].(string)
			if !ok {
				continue
			}
			if !p.offlineAccountSessionMatch(gatewayKey, accountSession) {
				resp = false
				continue
			}
			if currentAccount, ok := GAccountMgr.accounts.Find(p.aid); ok && currentAccount == p {
				GAccountMgr.accounts.Del(p.aid)
			}
			p.updateCharacterLogout()
			p.updateAccountRecord()
			p.characterManager.clearRuntime()
			p.gatewayKey = ""
			p.accountSession = ""
			resp = true
		case OnlineAccountActorCmdClientPacket:
			gateway, ok := msg.Args[0].(*Gateway)
			if !ok {
				continue
			}
			pkt, ok := msg.Args[1].(*pb.OnlineClientPacket)
			if !ok {
				continue
			}
			p.onClientPacket(gateway, pkt)
		case OnlineAccountActorCmdTeamScene:
			presence, ok := msg.Args[0].(sceneCharacterPresence)
			if !ok {
				continue
			}
			p.applyCharacterTeamSceneState(presence)
		case OnlineAccountActorCmdTryBindRoom:
			if len(msg.Args) != 1 {
				continue
			}
			input, ok := msg.Args[0].(combatRoomTryBindInput)
			if !ok {
				continue
			}
			resp = p.tryBindCombatRoom(input)
		case OnlineAccountActorCmdCapturePet:
			if len(msg.Args) != 1 {
				continue
			}
			input, ok := msg.Args[0].(combatRoomCaptureInput)
			if !ok {
				continue
			}
			resp = p.captureCombatPet(input)
		case OnlineAccountActorCmdRoomLeft:
			if len(msg.Args) != 1 {
				continue
			}
			input, ok := msg.Args[0].(combatRoomParticipantLeaveInput)
			if !ok {
				continue
			}
			resp = p.leaveCombatRoomParticipant(input)
		case OnlineAccountActorCmdRoomFinished:
			if len(msg.Args) != 1 {
				continue
			}
			finishInput, ok := msg.Args[0].(combatRoomFinishInput)
			if !ok || finishInput.combatRoom == nil || finishInput.gateway == nil || finishInput.result == nil {
				continue
			}
			characterUUID := finishInput.characterUUID
			combatRoom := finishInput.combatRoom
			gateway := finishInput.gateway
			result := finishInput.result
			character := p.characterManager.find(characterUUID)
			if result.GetRecipientCharacterUuid() != characterUUID {
				if character != nil && character.combatRoom == combatRoom {
					character.combatRoom = nil
					p.refreshCharacterPresence(character)
				}
				p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Internal.Code())
				resp = fmt.Errorf(
					"combat result recipient mismatch aid:%d character:%d recipient:%d",
					p.aid,
					characterUUID,
					result.GetRecipientCharacterUuid(),
				)
				continue
			}
			if character == nil || character.combatRoom != combatRoom {
				resp = true
				continue
			}
			characterKey := sceneCharacterKey{aid: p.aid, characterUUID: characterUUID}
			if combatResultDischargesCharacterTeam(result, characterKey) {
				p.dischargeCharacterTeam(characterKey)
			}
			if finishInput.rewardErr != nil {
				character.combatRoom = nil
				p.refreshCharacterPresence(character)
				p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Internal.Code())
				resp = finishInput.rewardErr
				continue
			}
			battleReward := finishInput.battleReward
			battleVictoryEnemyGroupID := uint32(0)
			if battleReward.victory {
				battleVictoryEnemyGroupID = finishInput.enemyGroupID
			}
			persistenceResult, persistErr := persistCombatParticipantResult(
				p.accountRecord,
				character.record,
				combatParticipantPersistenceInput{
					settledAtMs:               time.Now().UnixMilli(),
					battleVictoryEnemyGroupID: battleVictoryEnemyGroupID,
					characterExperience:       battleReward.characterExperience,
					settleDuelPoint:           battleReward.duelPointBattle,
					characterDuelPointDelta:   battleReward.characterDuelPointDelta,
					battlePetUUID:             battleReward.battlePetUUID,
					battlePetExperience:       battleReward.battlePetExperience,
					itemAssetIDs:              battleReward.itemAssetIDs,
				},
				func() error {
					return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
				},
			)
			if persistErr != nil {
				character.combatRoom = nil
				p.refreshCharacterPresence(character)
				p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Internal.Code())
				resp = persistErr
				continue
			}
			if persistenceResult.baseChanged {
				p.sendCharacterBaseChangedNotify(gateway, character.record)
			}
			if persistenceResult.changedPet != nil {
				p.sendCharacterPetChangedNotify(gateway, characterUUID, []*pb.PetRecord{persistenceResult.changedPet})
			}
			if len(persistenceResult.receivedItemFinalCountMap) > 0 {
				// 现有道具变化通知只同步可堆叠普通道具的最终数量. 装备实例
				// 已经写入权威背包, 但客户端装备增量协议尚未定义; 两类奖励
				// 均通过下方结构化战斗结算摘要展示.
				p.sendCharacterItemChangedNotify(
					gateway,
					characterUUID,
					persistenceResult.receivedItemFinalCountMap,
				)
			}
			if result.GetSettlement() != nil {
				// 每个接收角色使用房间最终结果的独立副本. 结算只追加真正
				// 写入当前账号档案的经验、DP和道具变化. 经验保持角色在前、
				// 战宠在后的稳定顺序; 客户端不能提交或覆盖奖励数值.
				settlementUnitKey := &pb.CombatUnitKey{
					Aid:           p.aid,
					CharacterUuid: characterUUID,
				}
				if persistenceResult.characterExperience.AppliedExp > 0 {
					result.Settlement.ExpRewardList = append(
						result.Settlement.ExpRewardList,
						&pb.CombatSettlementExpReward{
							UnitKey:  cloneCombatUnitKey(settlementUnitKey),
							ExpDelta: persistenceResult.characterExperience.AppliedExp,
						},
					)
				}
				if len(persistenceResult.changedTaskRecordMap) > 0 {
					p.sendCharacterTaskChangedNotify(gateway, characterUUID, persistenceResult.changedTaskRecordMap)
				}
				if persistenceResult.battlePetExperience.AppliedExp > 0 {
					result.Settlement.ExpRewardList = append(
						result.Settlement.ExpRewardList,
						&pb.CombatSettlementExpReward{
							UnitKey: &pb.CombatUnitKey{
								Aid:           settlementUnitKey.GetAid(),
								CharacterUuid: settlementUnitKey.GetCharacterUuid(),
								PetUuid:       battleReward.battlePetUUID,
							},
							ExpDelta: persistenceResult.battlePetExperience.AppliedExp,
						},
					)
				}
				if persistenceResult.duelPointChanged {
					result.Settlement.DuelPointChange = &pb.CombatSettlementDuelPointChange{
						UnitKey: cloneCombatUnitKey(settlementUnitKey),
						Delta:   persistenceResult.characterDuelPointDelta,
						After:   persistenceResult.characterDuelPointAfter,
					}
				}
				itemRewardMap := make(map[uint32]*pb.CombatSettlementItemReward)
				for _, itemAssetID := range persistenceResult.receivedItemAssetIDs {
					if reward := itemRewardMap[itemAssetID]; reward != nil {
						reward.Count++
						continue
					}
					reward := &pb.CombatSettlementItemReward{
						UnitKey: cloneCombatUnitKey(settlementUnitKey),
						ItemId:  itemAssetID,
						Count:   1,
					}
					itemRewardMap[itemAssetID] = reward
					result.Settlement.ItemRewardList = append(
						result.Settlement.ItemRewardList,
						reward,
					)
				}
			}
			character.combatRoom = nil
			p.refreshCharacterPresence(character)
			p.sendClientRes(gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Success.Code(), result)
			resp = true
		case OnlineAccountActorCmdStop:
			// Stop 也必须在 Account actor 内完成批量持久化和运行态清理,
			// 避免服务关闭与尚未处理完的角色请求并发读写同一聚合根.
			p.updateCharacterLogout()
			p.updateAccountRecord()
			p.characterManager.clearRuntime()
			p.gatewayKey = ""
			p.accountSession = ""
			resp = true
		}
	}
	return p.behavior, resp, nil
}

func (p *Account) offlineAccountSessionMatch(gatewayKey string, accountSession string) bool {
	if gatewayKey == "" || accountSession == "" || p.gatewayKey != gatewayKey {
		return false
	}
	return p.accountSession == accountSession
}

// updateCharacterLogoutTimestamp 将指定登出时间写入全部在线角色档案并持久化账号记录;
func (p *Account) updateCharacterLogout() {
	timestampMs := time.Now().UnixMilli()
	if p.characterManager == nil || p.accountRecord == nil {
		return
	}
	for _, character := range p.characterManager.characters {
		if character == nil || !character.online || character.record == nil {
			continue
		}
		character.record.Base.LastLogoutTimestampMs = timestampMs
	}
}

func (p *Account) updateAccountRecord() {
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after account offline failed aid:%d err:%v", p.aid, err)
	}
}
