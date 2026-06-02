package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	pb "server/proto/pb"

	xcontrol "github.com/75912001/xlib/control"
	xerror "github.com/75912001/xlib/error"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type EventCommand struct {
	Command string
}

func Bus(args ...any) error {
	value := args[0]
	var err error
	switch event := value.(type) {
	case *xnetcommon.Connect:
		err = event.IHandler.OnConnect(event.IRemote)
	case *xnetcommon.Packet:
		err = event.IHandler.OnPacket(event.IRemote, event.IPacket)
	case *xnetcommon.Disconnect:
		err = event.IHandler.OnDisconnect(event.IRemote)
	case *xcontrol.Event:
		if event.ISwitch.IsOff() {
			return nil
		}
		err = event.ICallBack.Execute()
	case *EventCommand:
		err = sendCommand(event.Command)
	default:
	}
	return err
}

func sendCommand(command string) error {
	data, err := loadAPI(apiYamlPath)
	if err != nil {
		ColorPrintf(Red, "load api yaml failed: %v\n", err)
		return err
	}
	apiData, ok := data[command]
	if !ok {
		ColorPrintf(Red, "%s\n", "api not found in api.yaml")
		return xerror.NotFound
	}
	num, err := strconv.ParseUint(apiData.ID, 0, 32)
	if err != nil {
		ColorPrintf(Red, "parse messageID failed: %v\n", err)
		return err
	}
	messageID := uint32(num)
	message := GMessage.Find(messageID)
	if message == nil {
		ColorPrintf(Red, "message not found: 0x%X\n", messageID)
		return xerror.NotFound
	}

	msgData := []byte("{}")
	if apiData.Msg != nil {
		msgData, err = json.Marshal(apiData.Msg)
		if err != nil {
			ColorPrintf(Red, "json marshal failed: %v\n", err)
			return err
		}
	}
	protoMsg, err := message.JsonUnmarshal(msgData)
	if err != nil {
		ColorPrintf(Red, "message json unmarshal failed: %v\n", err)
		return err
	}
	fillDynamicFields(GetClient(), protoMsg)

	if verifyReq, ok := protoMsg.(*pb.UserVerifyReq); ok {
		token, err := cacheSetVerifyUserToken(verifyReq.GetUid())
		if err != nil {
			ColorPrintf(Red, "set verify token failed: %v\n", err)
			return err
		}
		verifyReq.Token = token
		GetClient().uid = verifyReq.GetUid()
		GetClient().token = verifyReq.GetToken()
	}

	fmt.Println()
	ColorPrintf(Blue, "messageID: 0x%x\n", messageID)
	ColorPrintf(Blue, "Message: %s\n", marshalJSON(protoMsg))
	log.Infof("\n======send message======\n%s\nmessageID: 0x%x\nMessage: %s", command, messageID, marshalJSON(protoMsg))

	packet := &xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: messageID,
			SessionID: GetClient().nextSession,
			ResultID:  0,
			Key:       GetClient().uid,
		},
		PBMessage: protoMsg,
	}
	if GetClient().Remote == nil {
		return errors.WithMessage(xerror.Link, "remote is nil")
	}
	if err = GetClient().Remote.Send(packet); err != nil {
		ColorPrintf(Red, "client send failed: %v\n", err)
		log.Errorf("client send failed: %v", err)
	}
	return err
}

func fillDynamicFields(c *Client, msg proto.Message) {
	switch m := msg.(type) {
	case *pb.UserHeartbeatReq:
		if m.GetLastSession() == 0 {
			m.LastSession = c.nextSession
		}
	}
}
