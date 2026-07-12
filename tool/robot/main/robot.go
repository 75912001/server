package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	common "server/common"
	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xnetcommon "github.com/75912001/xlib/net/common"
	xnettcp "github.com/75912001/xlib/net/tcp"
	xpacket "github.com/75912001/xlib/packet"
	xtimer "github.com/75912001/xlib/timer"
)

type Robot struct {
	TCP    *xnettcp.Client
	Remote xnetcommon.IRemote

	manager *RobotManager

	aid               uint64
	gatewayAddr       string
	heartbeatSession  string
	connectTicket     string
	packetSessionID   uint32
	verified          bool
	accountReady      bool
	heartbeatWait     bool
	heartbeatPacketID uint32
	heartbeatTimerSeq uint64
	seq               uint64
	heartbeatTimer    *xtimer.Millisecond
	actionTimer       *xtimer.Millisecond
	ignoreMsgID       map[uint32]struct{}
	closeOnce         sync.Once
	closed            chan struct{}
	pending           []RobotPendingCommand
}

type RobotPendingCommand struct {
	Command string
	Verbose bool
	Source  string
}

func NewRobot(manager *RobotManager, aid uint64) *Robot {
	return &Robot{
		manager:     manager,
		aid:         aid,
		ignoreMsgID: buildIgnoreMsgID(GConfigYaml.IgnoreMsgID),
		closed:      make(chan struct{}),
	}
}

func (p *Robot) Start(ctx context.Context) error {
	if err := p.prepareLoginSession(ctx); err != nil {
		return err
	}
	p.TCP = xnettcp.NewClient(p)
	opts := xnettcp.NewConnectOptions().
		WithAddress(p.gatewayAddr).
		WithSendChanCapacity(uint32(GConfigYaml.Robot.SendChanCapacity)).
		WithHeaderStrategy(&common.DefaultHeaderStrategy{}).
		WithIOut(p.manager.iEventMgr)
	if err := p.TCP.Connect(ctx, opts); err != nil {
		return err
	}
	p.Remote = p.TCP.IRemote
	p.manager.iEventMgr.Send(&xcontrol.Event{
		ISwitch:   xcontrol.NewSwitchButton(true),
		ICallBack: xcontrol.NewCallBack(func(args ...any) error { return p.OnConnect(p.Remote) }),
	})
	p.manager.iEventMgr.Send(&RobotCommand{Robot: p, Command: "AccountVerifyReq", Source: "auto"})
	return nil
}

func (p *Robot) prepareLoginSession(ctx context.Context) error {
	account := robotAccount(p.aid)
	accountVerifyToken := newAccountVerifyToken(account)
	if err := loginCreateAccountVerifyToken(ctx, account, accountVerifyToken); err != nil {
		return err
	}
	session, err := loginUseAccountVerifyToken(ctx, account, accountVerifyToken)
	if err != nil {
		return err
	}
	if session.Aid == 0 || session.GatewayAddr == "" || session.ConnectTicket == "" {
		return fmt.Errorf("login session invalid aid=%d gatewayAddr=%q connectTicket=%t", session.Aid, session.GatewayAddr, session.ConnectTicket != "")
	}
	if session.Aid != p.aid {
		if err = p.manager.UpdateRobotAID(p, p.aid, session.Aid); err != nil {
			return err
		}
		p.aid = session.Aid
	}
	p.gatewayAddr = session.GatewayAddr
	p.connectTicket = session.ConnectTicket
	p.heartbeatSession = ""
	return nil
}

func (p *Robot) Close() {
	p.closeOnce.Do(func() {
		p.verified = false
		p.accountReady = false
		p.heartbeatWait = false
		p.heartbeatPacketID = 0
		p.gatewayAddr = ""
		p.heartbeatSession = ""
		p.connectTicket = ""
		p.pending = nil
		if p.Remote != nil && p.Remote.IsConnect() {
			p.Remote.Stop()
		}
		p.Remote = nil
		p.TCP = nil
		if xtimer.GTimer != nil && p.heartbeatTimer != nil {
			xtimer.GTimer.DelMillisecond(p.heartbeatTimer)
			p.heartbeatTimer = nil
		}
		p.heartbeatTimerSeq++
		if xtimer.GTimer != nil && p.actionTimer != nil {
			xtimer.GTimer.DelMillisecond(p.actionTimer)
			p.actionTimer = nil
		}
		close(p.closed)
	})
}

func (p *Robot) isClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}

func (p *Robot) OnConnect(remote xnetcommon.IRemote) error {
	p.manager.stats.connectOK.Add(1)
	if p.shouldPrintVerbose(false) {
		ColorPrintf(Green, "aid=%d connected addr=%s\n", p.aid, p.gatewayAddr)
	}
	return nil
}

func (p *Robot) OnCheckPacketLength(length uint32) error {
	if length < xpacket.HeaderSize || length > 65535 {
		return fmt.Errorf("invalid packet length=%d", length)
	}
	return nil
}

func (p *Robot) OnCheckPacketLimit(remote xnetcommon.IRemote) error {
	return nil
}

func (p *Robot) OnUnmarshalPacket(remote xnetcommon.IRemote, data []byte) (xpacket.IPacket, error) {
	header := xpacket.NewHeader()
	header.Unpack(data[:xpacket.HeaderSize])
	message := GMessage.Find(header.MessageID)
	if message == nil {
		ColorPrintf(Red, "aid=%d can not find message:%v, 0x%X\n", p.aid, header.MessageID, header.MessageID)
		return &xpacket.PacketPassThrough{
			Header:  header,
			RawData: append([]byte(nil), data...),
		}, nil
	}
	pbMsg, err := message.Unmarshal(data[xpacket.HeaderSize:])
	if err != nil {
		ColorPrintf(Red, "aid=%d can not unmarshal:%v, 0x%X err:%v\n", p.aid, header.MessageID, header.MessageID, err)
		return nil, err
	}
	return &xpacket.Packet{
		Header:    header,
		PBMessage: pbMsg,
		IMessage:  message,
	}, nil
}

func (p *Robot) OnPacket(remote xnetcommon.IRemote, packet xpacket.IPacket) error {
	switch pkt := packet.(type) {
	case *xpacket.Packet:
		return p.handleProtoPacket(pkt)
	case *xpacket.PacketPassThrough:
		p.manager.stats.received.Add(1)
		if pkt.Header.ResultID != 0 {
			p.manager.stats.resultFail.Add(1)
			ColorPrintf(Yellow, "aid=%d recv unknown messageID=0x%x resultID=%d len=%d\n", p.aid, pkt.Header.MessageID, pkt.Header.ResultID, len(pkt.RawData))
			ColorPrintf(Yellow, "aid=%d ResultError: %s\n", p.aid, formatResultError(pkt.Header.ResultID))
		}
		return nil
	default:
		return xerror.Mismatch
	}
}

func (p *Robot) handleProtoPacket(packet *xpacket.Packet) error {
	p.manager.stats.received.Add(1)
	if packet.Header.ResultID != 0 && !p.isExpectedNonZeroResult(packet) {
		p.manager.stats.resultFail.Add(1)
	}
	p.applyPacketState(packet)
	if !p.shouldPrintPacket(packet) {
		return nil
	}
	msgName := reflect.TypeOf(packet.PBMessage).Elem().Name()
	color := Green
	if packet.Header.ResultID != 0 {
		color = Yellow
	}
	fmt.Println()
	ColorPrintf(color, "aid=%d MessageName: %s\n", p.aid, msgName)
	headerJson, err := json.MarshalIndent(marshalHeaderMap(packet.Header), "", "  ")
	if err != nil {
		ColorPrintf(Red, "aid=%d json marshal header failed: %v\n", p.aid, err)
		return err
	}
	ColorPrintf(color, "Header: %s\n", headerJson)
	resultError := ""
	if packet.Header.ResultID != 0 {
		resultError = formatResultError(packet.Header.ResultID)
		ColorPrintf(color, "ResultError: %s\n", resultError)
	}
	body := marshalJSON(packet.PBMessage)
	ColorPrintf(color, "Message: %s\n", body)
	if resultError != "" {
		log.Infof("\n======recv message======\naid=%d\n%s\nHeader: %s\nResultError: %s\nMessage: %s", p.aid, msgName, headerJson, resultError, body)
	} else {
		log.Infof("\n======recv message======\naid=%d\n%s\nHeader: %s\nMessage: %s", p.aid, msgName, headerJson, body)
	}
	return nil
}

func (p *Robot) applyPacketState(packet *xpacket.Packet) {
	switch msg := packet.PBMessage.(type) {
	case *pb.AccountVerifyRes:
		if packet.Header.ResultID == 0 {
			p.heartbeatSession = msg.GetHeartbeatSession()
			if !p.verified {
				p.manager.stats.verifyOK.Add(1)
			}
			p.verified = true
			p.StartHeartBeat()
			p.manager.iEventMgr.Send(&RobotCommand{Robot: p, Command: "AccountRecordReq", Source: "auto"})
		} else {
			p.manager.stats.verifyFail.Add(1)
		}
	case *pb.AccountRecordRes:
		if packet.Header.ResultID == 0 {
			accountRecord := msg.GetAccountRecord()
			if accountRecordReady(accountRecord) {
				p.markAccountReady()
				return
			}
			return
		}
	case *pb.CharacterCreateRes:
		if packet.Header.ResultID == 0 || packet.Header.ResultID == xerror.AlreadyExists.Code() {
			p.markAccountReady()
		}
	case *pb.AccountHeartbeatRes:
		if !p.heartbeatWait || packet.Header.SessionID != p.heartbeatPacketID {
			return
		}
		p.heartbeatWait = false
		p.heartbeatPacketID = 0
		if packet.Header.ResultID == 0 {
			p.heartbeatSession = msg.GetNextHeartbeatSession()
			p.startHeartBeatTimer()
		}
	}
}

func accountRecordReady(accountRecord *pb.AccountRecord) bool {
	return accountRecord != nil && accountRecord.GetAid() != 0 && accountRecord.GetCreateTimestampMs() != 0
}

func (p *Robot) isExpectedNonZeroResult(packet *xpacket.Packet) bool {
	return packet.Header.MessageID == uint32(pb.MsgIDAccount_CharacterCreateRes_CMD) &&
		packet.Header.ResultID == xerror.AlreadyExists.Code()
}

func (p *Robot) markAccountReady() {
	if p.accountReady {
		return
	}
	p.accountReady = true
	p.StartAction()
	p.flushPendingCommands()
}

func (p *Robot) OnDisconnect(remote xnetcommon.IRemote) error {
	p.manager.stats.disconnect.Add(1)
	if p.shouldPrintVerbose(false) || GConfigYaml.Robot.Logging.DetailFailures {
		ColorPrintf(Red, "aid=%d OnDisconnect remote=%p reason=%d\n", p.aid, remote, remote.GetDisconnectReason())
	}
	log.Warnf("aid=%d OnDisconnect remote=%p reason=%d", p.aid, remote, remote.GetDisconnectReason())
	p.Close()
	return nil
}

func (p *Robot) shouldPrintVerbose(verbose bool) bool {
	return verbose || p.manager.Total() <= 1
}

func (p *Robot) shouldPrintPacket(packet *xpacket.Packet) bool {
	if packet.Header.ResultID != 0 {
		return GConfigYaml.Robot.Logging.DetailFailures || p.manager.Total() <= 1
	}
	if _, ok := p.ignoreMsgID[packet.Header.MessageID]; ok {
		return false
	}
	return p.manager.Total() <= 1
}

func (p *Robot) flushPendingCommands() {
	pending := p.pending
	p.pending = nil
	for _, command := range pending {
		_ = p.sendCommandNow(command.Command, command.Verbose, command.Source)
	}
}

func (p *Robot) View() robotView {
	connected := p.Remote != nil && p.Remote.IsConnect()
	return robotView{
		AID:              p.aid,
		GatewayAddr:      p.gatewayAddr,
		Connected:        connected,
		Verified:         p.verified,
		AccountReady:     p.accountReady,
		HeartbeatSession: p.heartbeatSession,
		Seq:              p.seq,
		Pending:          len(p.pending),
	}
}
