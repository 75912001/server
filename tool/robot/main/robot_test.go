package main

import (
	"strings"
	"testing"
	"time"

	common "server/common"
	pb "server/proto/pb"

	xevent "github.com/75912001/xlib/event"
	xpacket "github.com/75912001/xlib/packet"
)

func TestRobotAccountVerifyQueuesAccountRecordReq(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header:    &xpacket.Header{MessageID: uint32(pb.MsgIDAccount_AccountVerifyRes_CMD)},
		PBMessage: &pb.AccountVerifyRes{HeartbeatSession: "heartbeat-session"},
	})

	if got, want := readRobotCommand(t, commands), "AccountRecordReq"; got != want {
		t.Fatalf("auto command = %q, want %q", got, want)
	}
}

func TestRobotAccountRecordShellDoesNotAutoCreate(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header: &xpacket.Header{MessageID: uint32(pb.MsgIDAccount_AccountRecordRes_CMD)},
		PBMessage: &pb.AccountRecordRes{AccountRecord: &pb.AccountRecord{
			Aid:                      10001,
			Account:                  "robot.10001",
			AccountCreateTimestampMs: 123,
		}},
	})

	assertNoRobotCommand(t, commands)
	if robot.accountReady {
		t.Fatal("robot accountReady = true, want false")
	}
}

func TestRobotAccountRecordNotCreatedResultDoesNotAutoCreate(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDAccount_AccountRecordRes_CMD),
			ResultID:  common.ECOnlineAccountNotCreated.Code(),
		},
		PBMessage: &pb.AccountRecordRes{},
	})

	assertNoRobotCommand(t, commands)
	if robot.accountReady {
		t.Fatal("robot accountReady = true, want false")
	}
}

func TestRobotCreatedAccountRecordMarksReady(t *testing.T) {
	robot, commands := newRobotForStateTest(t)

	robot.applyPacketState(&xpacket.Packet{
		Header: &xpacket.Header{MessageID: uint32(pb.MsgIDAccount_AccountRecordRes_CMD)},
		PBMessage: &pb.AccountRecordRes{AccountRecord: &pb.AccountRecord{
			Aid:                            10001,
			Account:                        "robot.10001",
			AccountCreateTimestampMs:       123,
			AccountRecordCreateTimestampMs: 456,
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
	if !strings.Contains(data, `"carryStatus": 2`) {
		t.Fatalf("marshalJSON carryStatus output = %s, want carryStatus enum number", data)
	}
	if !strings.Contains(data, `"grade": 5`) {
		t.Fatalf("marshalJSON grade output = %s, want grade enum number", data)
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
