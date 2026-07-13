package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "server/proto/pb"

	xevent "github.com/75912001/xlib/event"
	xpacket "github.com/75912001/xlib/packet"
)

func TestRobotAccountVerifyQueuesAccountRecordReq(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header:    &xpacket.Header{MessageID: uint32(pb.MsgID_AccountVerifyRes_CMD)},
		PBMessage: &pb.AccountVerifyRes{HeartbeatSession: "heartbeat-session"},
	})

	if got, want := readRobotCommand(t, commands), "AccountRecordReq"; got != want {
		t.Fatalf("auto command = %q, want %q", got, want)
	}
}

func TestRobotAccountRecordMarksReady(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header: &xpacket.Header{MessageID: uint32(pb.MsgID_AccountRecordRes_CMD)},
		PBMessage: &pb.AccountRecordRes{AccountRecord: &pb.AccountRecord{
			Aid:               10001,
			Account:           "robot.10001",
			CreateTimestampMs: 123,
		}},
	})

	assertNoRobotCommand(t, commands)
	if !robot.accountReady {
		t.Fatal("robot accountReady = false, want true")
	}
}

func TestMarshalJSONUsesEnumNumbers(t *testing.T) {
	data := marshalJSON(&pb.PetRecord{
		CarryStatus: pb.PetCarryStatus_PetCarryStatus_Battle,
		Grade:       pb.PetGrade_PetGrade_Mythic,
	})

	if strings.Contains(data, "PetCarryStatus_Battle") || strings.Contains(data, "PetGrade_Mythic") {
		t.Fatalf("marshalJSON enum output = %s, want enum numbers", data)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("marshalJSON output is invalid JSON: %v, data:%s", err, data)
	}
	if value, ok := got["carryStatus"].(float64); !ok || int(value) != int(pb.PetCarryStatus_PetCarryStatus_Battle) {
		t.Fatalf("marshalJSON carryStatus = %v, want %d", got["carryStatus"], pb.PetCarryStatus_PetCarryStatus_Battle)
	}
	if value, ok := got["grade"].(float64); !ok || int(value) != int(pb.PetGrade_PetGrade_Mythic) {
		t.Fatalf("marshalJSON grade = %v, want %d", got["grade"], pb.PetGrade_PetGrade_Mythic)
	}
}

func newRobotForStateTest(t *testing.T) (*Robot, <-chan string) {
	t.Helper()

	commands := make(chan string, 4)
	manager := &RobotManager{
		iEventMgr: xevent.NewListMgr(1, func(args ...any) error {
			if len(args) == 0 {
				return nil
			}
			command, ok := args[0].(*RobotCommand)
			if ok {
				commands <- command.Command
			}
			return nil
		}),
		stats:  &RobotStats{},
		closed: make(chan struct{}),
	}
	manager.StartEventLoop()
	t.Cleanup(manager.Stop)

	return &Robot{
		manager:     manager,
		aid:         10001,
		ignoreMsgID: map[uint32]struct{}{},
		closed:      make(chan struct{}),
	}, commands
}

func readRobotCommand(t *testing.T, commands <-chan string) string {
	t.Helper()

	select {
	case command := <-commands:
		return command
	case <-time.After(time.Second):
		t.Fatal("timeout waiting robot command")
		return ""
	}
}

func assertNoRobotCommand(t *testing.T, commands <-chan string) {
	t.Helper()

	select {
	case command := <-commands:
		t.Fatalf("unexpected auto command: %s", command)
	case <-time.After(20 * time.Millisecond):
	}
}
