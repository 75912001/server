package main

import (
	"context"
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

// PostCombatRoomFinishedSync 同步请求 Account actor 清除匹配房间引用并投递最终战报.
func (p *Account) PostCombatRoomFinishedSync(characterUUID uint64, room *CombatRoom, gateway *Gateway, result *pb.CombatRoundResultNotify) error {
	resp, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdRoomFinished, characterUUID, room, gateway, result))
	if err != nil {
		return err
	}
	if finishErr, ok := resp.(error); ok {
		return finishErr
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
		case OnlineAccountActorCmdRoomFinished:
			if len(msg.Args) != 4 {
				continue
			}
			characterUUID, characterUUIDOK := msg.Args[0].(uint64)
			room, roomOK := msg.Args[1].(*CombatRoom)
			gateway, gatewayOK := msg.Args[2].(*Gateway)
			result, resultOK := msg.Args[3].(*pb.CombatRoundResultNotify)
			if !characterUUIDOK || !roomOK || !gatewayOK || !resultOK {
				continue
			}
			character := p.characterManager.find(characterUUID)
			if character == nil || character.combatRoom != room {
				resp = true
				continue
			}
			participantKey := combatRoomParticipantKey{
				aid:           p.aid,
				characterUUID: characterUUID,
			}
			battleReward, rewardErr := room.playerCombatBattleReward(
				participantKey,
				result.GetSettlement().GetBattleResult(),
			)
			if rewardErr != nil {
				character.combatRoom = nil
				p.sendClientErr(gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Internal.Code())
				resp = rewardErr
				continue
			}
			persistenceResult, persistErr := persistCombatParticipantResult(
				p.accountRecord,
				character.record,
				combatParticipantPersistenceInput{
					characterExperience:     battleReward.characterExperience,
					settleDuelPoint:         battleReward.duelPointBattle,
					characterDuelPointDelta: battleReward.characterDuelPointDelta,
					battlePetUUID:           battleReward.battlePetUUID,
					battlePetExperience:     battleReward.battlePetExperience,
					itemAssetIDs:            battleReward.itemAssetIDs,
				},
				func() error {
					return unaryCacheSetAccountRecord(p.aid, p.accountRecord)
				},
			)
			if persistErr != nil {
				character.combatRoom = nil
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
