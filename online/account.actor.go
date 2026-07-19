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
	_, err := p.actor.SendMsgSync(xactor.NewMsg(context.Background(), OnlineAccountActorCmdRoomFinished, characterUUID, room, gateway, result))
	return err
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
		character.record.LastLogoutTimestampMs = timestampMs
	}
}

func (p *Account) updateAccountRecord() {
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after account offline failed aid:%d err:%v", p.aid, err)
	}
}
