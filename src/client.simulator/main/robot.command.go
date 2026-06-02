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
	case "uid":
		p.executeUIDCommand(parts)
	default:
		robots := p.Robots()
		if len(robots) == 1 {
			p.iEventMgr.Send(&RobotCommand{Robot: robots[0], Command: parts[0], Verbose: true, Source: "manual"})
			return false
		}
		ColorPrintf(Yellow, "use command: all <MessageName> or uid <UID> <MessageName>\n")
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

func (p *RobotManager) executeUIDCommand(parts []string) {
	if len(parts) < 3 {
		ColorPrintf(Yellow, "usage: uid <UID> <MessageName>\n")
		return
	}
	uid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		ColorPrintf(Red, "parse uid failed: %v\n", err)
		return
	}
	if err := p.QueueUIDCommand(uid, parts[2]); err != nil {
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

func (p *RobotManager) QueueUIDCommand(uid uint64, command string) error {
	robot, ok := p.Find(uid)
	if !ok {
		return errors.Errorf("robot not found uid=%d", uid)
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
		return errors.WithMessagef(xerror.Disconnect, "robot closed uid=%d", p.uid)
	}
	if event.Command != "UserVerifyReq" && !p.verified {
		p.pending = append(p.pending, RobotPendingCommand{
			Command: event.Command,
			Verbose: event.Verbose,
			Source:  event.Source,
		})
		p.manager.stats.queued.Add(1)
		if p.shouldPrintVerbose(event.Verbose) {
			ColorPrintf(Yellow, "uid=%d not verified, queued command=%s\n", p.uid, event.Command)
		}
		return nil
	}
	if requiresUserReady(event.Command) && !p.userReady {
		p.pending = append(p.pending, RobotPendingCommand{
			Command: event.Command,
			Verbose: event.Verbose,
			Source:  event.Source,
		})
		p.manager.stats.queued.Add(1)
		if p.shouldPrintVerbose(event.Verbose) {
			ColorPrintf(Yellow, "uid=%d user not created, queued command=%s\n", p.uid, event.Command)
		}
		return nil
	}
	if event.Source == "heartbeat" && event.Command == "UserHeartbeatReq" {
		if p.heartbeatWait {
			return nil
		}
		p.heartbeatWait = true
		p.heartbeatSession = p.nextSession
	}
	err := p.sendCommandNow(event.Command, event.Verbose, event.Source)
	if err != nil && event.Source == "heartbeat" {
		p.heartbeatWait = false
		p.heartbeatSession = 0
	}
	return err
}

func (p *Robot) sendCommandNow(command string, verbose bool, source string) error {
	data, err := loadAPI(apiYamlPath)
	if err != nil {
		p.onCommandError(verbose, "uid=%d load api yaml failed: %v", p.uid, err)
		return err
	}
	apiData, ok := data[command]
	if !ok {
		err = xerror.NotFound
		p.onCommandError(verbose, "uid=%d api not found in api.yaml command=%s", p.uid, command)
		return err
	}
	num, err := strconv.ParseUint(apiData.ID, 0, 32)
	if err != nil {
		p.onCommandError(verbose, "uid=%d parse messageID failed: %v", p.uid, err)
		return err
	}
	messageID := uint32(num)
	message := GMessage.Find(messageID)
	if message == nil {
		err = xerror.NotFound
		p.onCommandError(verbose, "uid=%d message not found: 0x%X", p.uid, messageID)
		return err
	}

	msgData := []byte("{}")
	if apiData.Msg != nil {
		msgData, err = json.Marshal(apiData.Msg)
		if err != nil {
			p.onCommandError(verbose, "uid=%d json marshal failed: %v", p.uid, err)
			return err
		}
	}
	protoMsg, err := message.JsonUnmarshal(msgData)
	if err != nil {
		p.onCommandError(verbose, "uid=%d message json unmarshal failed command=%s err=%v", p.uid, command, err)
		return err
	}
	if err = p.fillDynamicFields(protoMsg); err != nil {
		p.onCommandError(verbose, "uid=%d fill dynamic fields failed command=%s err=%v", p.uid, command, err)
		return err
	}

	if p.shouldPrintVerbose(verbose) {
		fmt.Println()
		ColorPrintf(Blue, "uid=%d messageID: 0x%x\n", p.uid, messageID)
		ColorPrintf(Blue, "Message: %s\n", marshalJSON(protoMsg))
		log.Infof("\n======send message======\nuid=%d\n%s\nmessageID: 0x%x\nMessage: %s", p.uid, command, messageID, marshalJSON(protoMsg))
	}

	packet := &xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: messageID,
			SessionID: p.nextSession,
			ResultID:  0,
			Key:       p.uid,
		},
		PBMessage: protoMsg,
	}
	if p.Remote == nil || !p.Remote.IsConnect() {
		err = errors.WithMessage(xerror.Link, "remote is nil or disconnected")
		p.onCommandError(verbose, "uid=%d client send failed: %v", p.uid, err)
		p.manager.stats.sendFail.Add(1)
		return err
	}
	if err = p.Remote.Send(packet); err != nil {
		p.onCommandError(verbose, "uid=%d client send failed: %v", p.uid, err)
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
	case *pb.UserVerifyReq:
		token, err := cacheSetVerifyUserToken(p.uid)
		if err != nil {
			return err
		}
		m.Uid = p.uid
		m.Token = token
		p.token = token
	case *pb.UserHeartbeatReq:
		if m.GetLastSession() == 0 {
			m.LastSession = p.nextSession
		}
	case *pb.RobotPingReq:
		if m.GetSeq() == 0 {
			p.seq++
			m.Seq = p.seq
		}
		if m.GetClientTime() == 0 {
			m.ClientTime = time.Now().UnixMilli()
		}
	}
	return nil
}

func (p *Robot) onCommandError(verbose bool, format string, a ...any) {
	p.manager.stats.commandError.Add(1)
	if p.shouldPrintVerbose(verbose) || GConfigYaml.Robot.Logging.DetailFailures {
		ColorPrintf(Red, format+"\n", a...)
	}
	log.Errorf(format, a...)
}

func requiresUserReady(command string) bool {
	return command == "RobotPingReq"
}
