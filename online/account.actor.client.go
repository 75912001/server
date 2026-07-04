package main

import (
	"errors"
	"server/common"
	pb "server/proto/pb"
	"time"

	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/protobuf/proto"
)

func (p *Account) onClientPacket(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	msgID := pb.MsgIDAccount(pkt.GetMessageId())
	switch msgID {
	case pb.MsgIDAccount_AccountRecordReq_CMD:
		p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_AccountRecordRes_CMD), xerror.Success.Code(),
			&pb.AccountRecordRes{
				AccountRecord: p.accountRecord,
			},
		)
		return
	case pb.MsgIDAccount_CharacterCreateReq_CMD:
		p.onCharacterCreateReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CharacterOnlineReq_CMD:
		p.onCharacterOnlineReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CharacterOfflineReq_CMD:
		p.onCharacterOfflineReq(gateway, pkt)
		return
	case pb.MsgIDAccount_SceneEnterReq_CMD:
		p.onSceneEnterReq(gateway, pkt)
		return
	default:
		if p.accountRecord == nil || p.accountRecord.GetAccountRecordCreateTimestampMs() == 0 {
			p.sendClientErr(gateway, pkt, uint32(msgID), common.ECOnlineAccountNotCreated.Code())
			return
		}
		// xlog.GLog.Warnf("unknown client packet aid:%d messageID:%d", p.aid, pkt.GetMessageId())
	}
	switch msgID {
	case pb.MsgIDAccount_AutoEncounterSetReq_CMD:
		p.onAutoEncounterSetReq(gateway, pkt)
		return
	case pb.MsgIDAccount_CombatRoundActionReq_CMD:
		p.onCombatRoundActionReq(gateway, pkt)
		return
	case pb.MsgIDAccount_RobotPingReq_CMD:
		p.onRobotPingReq(gateway, pkt)
		return
	default:
		xlog.GLog.Warnf("unknown client packet aid:%d messageID:%d", p.aid, pkt.GetMessageId())
		return
	}
}

func (p *Account) onRobotPingReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.RobotPingReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_RobotPingRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_RobotPingRes_CMD), xerror.Success.Code(),
		&pb.RobotPingRes{
			Seq:               req.GetSeq(),
			ClientTimestampMs: req.GetClientTimestampMs(),
			ServerTimestampMs: time.Now().UnixMilli(),
			Payload:           req.GetPayload(),
		},
	)
}

func (p *Account) onCharacterCreateReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterCreateReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	if p.accountRecord == nil {
		xlog.GLog.Errorf("account record is nil aid:%d", p.aid)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}

	now := time.Now().UnixMilli()
	if err := initializeDefaultAccountRecord(p.accountRecord, req.GetCharacterSlotIndex(), req.GetCharacterId(), req.GetCharacterNick(), req.GetCharacterElemental(), req.GetCharacterAttribute(), now); err != nil {
		if errors.Is(err, errCharacterSlotIndexInvalid) ||
			errors.Is(err, errCharacterIDInvalid) ||
			errors.Is(err, errCharacterNickInvalid) ||
			errors.Is(err, errCharacterElementalInvalid) ||
			errors.Is(err, errCharacterAttributeInvalid) {
			p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.InvalidArgument.Code())
			return
		}
		if errors.Is(err, errCharacterSlotOccupied) {
			p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.AlreadyExists.Code())
			return
		}
		xlog.GLog.Errorf("initialize account record failed aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record failed aid:%d err:%v", p.aid, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Internal.Code())
		return
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterCreateRes_CMD), xerror.Success.Code(), &pb.CharacterCreateRes{
		AccountRecord: p.accountRecord,
	})
}

func (p *Account) onCharacterOnlineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOnlineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if !p.accountRecordReady() {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), common.ECOnlineAccountNotCreated.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	if characterUUID == 0 {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.findCharacterRecord(characterUUID)
	if character == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.NotFound.Code())
		return
	}
	if p.isCharacterOnline(characterUUID) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.AlreadyExists.Code())
		return
	}
	if _, err := p.characterSceneID(character); err != nil {
		xlog.GLog.Warnf("character scene invalid aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	rollbackLastLoginTimestamp := setCharacterLastLoginTimestamp(character, time.Now().UnixMilli())
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		rollbackLastLoginTimestamp()
		xlog.GLog.Errorf("set account record after character online failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.Internal.Code())
		return
	}
	p.ensureOnlineCharacterUUIDSet()
	p.onlineCharacterUUIDSet[characterUUID] = struct{}{}
	p.activeCharacterUUID = characterUUID
	p.autoEncounterEnabled = false
	p.clearAutoEncounterTimer()
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOnlineRes_CMD), xerror.Success.Code(), &pb.CharacterOnlineRes{})
}

func setCharacterLastLoginTimestamp(character *pb.CharacterRecord, timestampMs int64) func() {
	previousValue := character.LastLoginTimestampMs
	character.LastLoginTimestampMs = timestampMs
	return func() {
		character.LastLoginTimestampMs = previousValue
	}
}

func setCharacterLastLogoutTimestamp(character *pb.CharacterRecord, timestampMs int64) func() {
	previousValue := character.LastLogoutTimestampMs
	character.LastLogoutTimestampMs = timestampMs
	return func() {
		character.LastLogoutTimestampMs = previousValue
	}
}

func setCharacterSceneID(character *pb.CharacterRecord, sceneID uint32) func() {
	previousValue := character.SceneId
	character.SceneId = sceneID
	return func() {
		character.SceneId = previousValue
	}
}

func (p *Account) onSceneEnterReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.SceneEnterReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if !p.accountRecordReady() {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), common.ECOnlineAccountNotCreated.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	if characterUUID == 0 || req.GetSceneId() == 0 {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.findCharacterRecord(characterUUID)
	if character == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.NotFound.Code())
		return
	}
	if !p.isCharacterOnline(characterUUID) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if p.combatState != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	if !isValidSceneID(req.GetSceneId()) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.InvalidArgument.Code())
		return
	}

	rollbackSceneID := setCharacterSceneID(character, req.GetSceneId())
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		rollbackSceneID()
		xlog.GLog.Errorf("set account record after scene enter failed aid:%d character:%d scene:%d err:%v", p.aid, characterUUID, req.GetSceneId(), err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.Internal.Code())
		return
	}
	p.activeCharacterUUID = characterUUID
	p.autoEncounterEnabled = false
	p.clearAutoEncounterTimer()
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_SceneEnterRes_CMD), xerror.Success.Code(), &pb.SceneEnterRes{
		CharacterUuid:     characterUUID,
		SceneId:           req.GetSceneId(),
		ServerTimestampMs: time.Now().UnixMilli(),
	})
}

func (p *Account) onCharacterOfflineReq(gateway *Gateway, pkt *pb.OnlineClientPacket) {
	var req pb.CharacterOfflineReq
	if err := proto.Unmarshal(pkt.GetBody(), &req); err != nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	if !p.accountRecordReady() {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), common.ECOnlineAccountNotCreated.Code())
		return
	}
	characterUUID := req.GetCharacterUuid()
	if characterUUID == 0 {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.InvalidArgument.Code())
		return
	}
	character := p.findCharacterRecord(characterUUID)
	if character == nil {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.NotFound.Code())
		return
	}
	if !p.isCharacterOnline(characterUUID) {
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.FailedPrecondition.Code())
		return
	}
	rollbackLastLogoutTimestamp := setCharacterLastLogoutTimestamp(character, time.Now().UnixMilli())
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		rollbackLastLogoutTimestamp()
		xlog.GLog.Errorf("set account record after character offline failed aid:%d character:%d err:%v", p.aid, characterUUID, err)
		p.sendClientErr(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.Internal.Code())
		return
	}
	delete(p.onlineCharacterUUIDSet, characterUUID)
	if p.activeCharacterUUID == characterUUID {
		p.activeCharacterUUID = 0
	}
	p.sendClientRes(gateway, pkt, uint32(pb.MsgIDAccount_CharacterOfflineRes_CMD), xerror.Success.Code(), &pb.CharacterOfflineRes{})
}

func (p *Account) accountRecordReady() bool {
	return p.accountRecord != nil && p.accountRecord.GetAccountRecordCreateTimestampMs() != 0
}

func (p *Account) findCharacterRecord(characterUUID uint64) *pb.CharacterRecord {
	if characterUUID == 0 || p.accountRecord == nil {
		return nil
	}
	for _, character := range p.accountRecord.GetCharacterRecordList() {
		if character != nil && character.GetUuid() == characterUUID {
			return character
		}
	}
	return nil
}

func (p *Account) ensureOnlineCharacterUUIDSet() {
	if p.onlineCharacterUUIDSet == nil {
		p.onlineCharacterUUIDSet = make(map[uint64]struct{})
	}
}

func (p *Account) isCharacterOnline(characterUUID uint64) bool {
	if characterUUID == 0 || p.onlineCharacterUUIDSet == nil {
		return false
	}
	_, ok := p.onlineCharacterUUIDSet[characterUUID]
	return ok
}

func (p *Account) clearOnlineCharacterUUIDs() {
	if len(p.onlineCharacterUUIDSet) > 0 {
		p.onlineCharacterUUIDSet = make(map[uint64]struct{})
	}
	p.activeCharacterUUID = 0
}

func (p *Account) persistOnlineCharacterLogoutTimestamp(timestampMs int64) {
	if len(p.onlineCharacterUUIDSet) == 0 || p.accountRecord == nil {
		return
	}
	updated := false
	for characterUUID := range p.onlineCharacterUUIDSet {
		character := p.findCharacterRecord(characterUUID)
		if character == nil {
			xlog.GLog.Warnf("online character missing during account offline aid:%d character:%d", p.aid, characterUUID)
			continue
		}
		setCharacterLastLogoutTimestamp(character, timestampMs)
		updated = true
	}
	if !updated {
		return
	}
	if err := unaryCacheSetAccountRecord(p.aid, p.accountRecord); err != nil {
		xlog.GLog.Errorf("set account record after account offline failed aid:%d err:%v", p.aid, err)
	}
}

func (p *Account) sendClientRes(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32, message proto.Message) {
	body, err := proto.Marshal(message)
	if err != nil {
		xlog.GLog.Errorf("marshal client response failed aid:%d messageID:%d err:%v", p.aid, messageID, err)
		return
	}
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: pkt.GetSessionId(),
				ResultId:  resultID,
				Key:       p.aid,
				Body:      body,
			},
		},
	})
}

func (p *Account) sendClientErr(gateway *Gateway, pkt *pb.OnlineClientPacket, messageID uint32, resultID uint32) {
	gateway.Send(&pb.OnlineTunnelFrame{
		Aid: p.aid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: messageID,
				SessionId: pkt.GetSessionId(),
				ResultId:  resultID,
				Key:       p.aid,
				Body:      nil,
			},
		},
	})
}
