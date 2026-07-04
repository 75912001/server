package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xpacket "github.com/75912001/xlib/packet"
	"github.com/pkg/errors"
)

type RobotCommand struct {
	Robot   *Robot
	Command string
	Verbose bool
	Source  string
}

func (p *RobotManager) ExecuteCommand(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "quit", "exit":
		p.Stop()
		return true
	case "list":
		printAPIList()
	case "stats":
		p.PrintStats()
	case "all":
		p.executeAllCommand(parts)
	case "aid":
		p.executeAIDCommand(parts)
	default:
		robots := p.Robots()
		if len(robots) == 1 {
			p.iEventMgr.Send(&RobotCommand{Robot: robots[0], Command: parts[0], Verbose: true, Source: "manual"})
			return false
		}
		ColorPrintf(Yellow, "use command: all <MessageName> or aid <AID> <MessageName>\n")
	}
	return false
}

func (p *RobotManager) executeAllCommand(parts []string) {
	if len(parts) < 2 {
		ColorPrintf(Yellow, "usage: all <MessageName>\n")
		return
	}
	command := parts[1]
	queued := p.QueueAllCommand(command)
	ColorPrintf(Cyan, "queued command=%s robots=%d\n", command, queued)
}

func (p *RobotManager) executeAIDCommand(parts []string) {
	if len(parts) < 3 {
		ColorPrintf(Yellow, "usage: aid <AID> <MessageName>\n")
		return
	}
	aid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		ColorPrintf(Red, "parse aid failed: %v\n", err)
		return
	}
	if err := p.QueueAIDCommand(aid, parts[2]); err != nil {
		ColorPrintf(Red, "%v\n", err)
	}
}

func (p *RobotManager) QueueAllCommand(command string) int {
	robots := p.Robots()
	verbose := len(robots) == 1
	for _, robot := range robots {
		p.iEventMgr.Send(&RobotCommand{Robot: robot, Command: command, Verbose: verbose, Source: "manual"})
	}
	return len(robots)
}

func (p *RobotManager) QueueAIDCommand(aid uint64, command string) error {
	robot, ok := p.Find(aid)
	if !ok {
		return errors.Errorf("robot not found aid=%d", aid)
	}
	p.iEventMgr.Send(&RobotCommand{Robot: robot, Command: command, Verbose: true, Source: "manual"})
	return nil
}

func (p *Robot) SendCommand(event *RobotCommand) error {
	if event == nil {
		return xerror.InvalidArgument
	}
	if p.isClosed() {
		if event.Source != "manual" {
			return nil
		}
		p.manager.stats.commandError.Add(1)
		return errors.WithMessagef(xerror.Disconnect, "robot closed aid=%d", p.aid)
	}
	if event.Command != "AccountVerifyReq" && !p.verified {
		p.pending = append(p.pending, RobotPendingCommand{
			Command: event.Command,
			Verbose: event.Verbose,
			Source:  event.Source,
		})
		p.manager.stats.queued.Add(1)
		if p.shouldPrintVerbose(event.Verbose) {
			ColorPrintf(Yellow, "aid=%d not verified, queued command=%s\n", p.aid, event.Command)
		}
		return nil
	}
	if requiresAccountReady(event.Command) && !p.accountReady {
		p.pending = append(p.pending, RobotPendingCommand{
			Command: event.Command,
			Verbose: event.Verbose,
			Source:  event.Source,
		})
		p.manager.stats.queued.Add(1)
		if p.shouldPrintVerbose(event.Verbose) {
			ColorPrintf(Yellow, "aid=%d account not created, queued command=%s\n", p.aid, event.Command)
		}
		return nil
	}
	if event.Source == "heartbeat" && event.Command == "AccountHeartbeatReq" {
		if p.heartbeatWait {
			return nil
		}
		p.heartbeatWait = true
	}
	err := p.sendCommandNow(event.Command, event.Verbose, event.Source)
	if err != nil && event.Source == "heartbeat" {
		p.heartbeatWait = false
		p.heartbeatPacketID = 0
	}
	return err
}

func (p *Robot) sendCommandNow(command string, verbose bool, source string) error {
	data, err := loadAPI(apiYamlPath)
	if err != nil {
		p.onCommandError(verbose, "aid=%d load api yaml failed: %v", p.aid, err)
		return err
	}
	apiData, ok := data[command]
	if !ok {
		err = xerror.NotFound
		p.onCommandError(verbose, "aid=%d api not found in api.yaml command=%s", p.aid, command)
		return err
	}
	num, err := strconv.ParseUint(apiData.ID, 0, 32)
	if err != nil {
		p.onCommandError(verbose, "aid=%d parse messageID failed: %v", p.aid, err)
		return err
	}
	messageID := uint32(num)
	message := GMessage.Find(messageID)
	if message == nil {
		err = xerror.NotFound
		p.onCommandError(verbose, "aid=%d message not found: 0x%X", p.aid, messageID)
		return err
	}

	msgData := []byte("{}")
	if apiData.Msg != nil {
		msgData, err = json.Marshal(apiData.Msg)
		if err != nil {
			p.onCommandError(verbose, "aid=%d json marshal failed: %v", p.aid, err)
			return err
		}
	}
	protoMsg, err := message.JsonUnmarshal(msgData)
	if err != nil {
		p.onCommandError(verbose, "aid=%d message json unmarshal failed command=%s err=%v", p.aid, command, err)
		return err
	}
	if err = p.fillDynamicFields(protoMsg); err != nil {
		p.onCommandError(verbose, "aid=%d fill dynamic fields failed command=%s err=%v", p.aid, command, err)
		return err
	}

	if p.shouldPrintVerbose(verbose) {
		fmt.Println()
		ColorPrintf(Blue, "aid=%d messageID: 0x%x\n", p.aid, messageID)
		ColorPrintf(Blue, "Message: %s\n", marshalJSON(protoMsg))
		log.Infof("\n======send message======\naid=%d\n%s\nmessageID: 0x%x\nMessage: %s", p.aid, command, messageID, marshalJSON(protoMsg))
	}

	sessionID := p.nextPacketSessionID()
	if source == "heartbeat" {
		p.heartbeatPacketID = sessionID
	}
	packet := &xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: messageID,
			SessionID: sessionID,
			ResultID:  0,
			Key:       p.aid,
		},
		PBMessage: protoMsg,
	}
	if p.Remote == nil || !p.Remote.IsConnect() {
		err = errors.WithMessage(xerror.Link, "remote is nil or disconnected")
		p.onCommandError(verbose, "aid=%d client send failed: %v", p.aid, err)
		p.manager.stats.sendFail.Add(1)
		return err
	}
	if err = p.Remote.Send(packet); err != nil {
		p.onCommandError(verbose, "aid=%d client send failed: %v", p.aid, err)
		p.manager.stats.sendFail.Add(1)
		return err
	}
	p.manager.stats.sent.Add(1)
	if source == "action" {
		p.manager.stats.actionSent.Add(1)
	}
	return nil
}

func (p *Robot) fillDynamicFields(msg any) error {
	switch m := msg.(type) {
	case *pb.AccountVerifyReq:
		m.Aid = p.aid
		if p.connectTicket != "" {
			m.ConnectTicket = p.connectTicket
		}
	case *pb.AccountHeartbeatReq:
		if m.GetLastHeartbeatSession() == "" {
			m.LastHeartbeatSession = p.heartbeatSession
		}
	case *pb.RobotPingReq:
		if m.GetSeq() == 0 {
			p.seq++
			m.Seq = p.seq
		}
		if m.GetClientTimestampMs() == 0 {
			m.ClientTimestampMs = time.Now().UnixMilli()
		}
	}
	return nil
}

func (p *Robot) nextPacketSessionID() uint32 {
	p.packetSessionID++
	if p.packetSessionID == 0 {
		p.packetSessionID++
	}
	return p.packetSessionID
}

func (p *Robot) onCommandError(verbose bool, format string, a ...any) {
	p.manager.stats.commandError.Add(1)
	if p.shouldPrintVerbose(verbose) || GConfigYaml.Robot.Logging.DetailFailures {
		ColorPrintf(Red, format+"\n", a...)
	}
	log.Errorf(format, a...)
}

func requiresAccountReady(command string) bool {
	return command == "RobotPingReq"
}
