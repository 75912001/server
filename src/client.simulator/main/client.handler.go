package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

func (p *Client) OnConnect(remote xnetcommon.IRemote) error {
	ColorPrintf(Green, "connected addr=%s\n", GetClient().gatewayAddr)
	return nil
}

func (p *Client) OnCheckPacketLength(length uint32) error {
	if length < xpacket.HeaderSize || length > 65535 {
		return fmt.Errorf("invalid packet length=%d", length)
	}
	return nil
}

func (p *Client) OnCheckPacketLimit(remote xnetcommon.IRemote) error {
	return nil
}

func (p *Client) OnUnmarshalPacket(remote xnetcommon.IRemote, data []byte) (xpacket.IPacket, error) {
	header := xpacket.NewHeader()
	header.Unpack(data[:xpacket.HeaderSize])
	message := GMessage.Find(header.MessageID)
	if message == nil {
		ColorPrintf(Red, "can not find message:%v, 0x%X\n", header.MessageID, header.MessageID)
		return &xpacket.PacketPassThrough{
			Header:  header,
			RawData: append([]byte(nil), data...),
		}, nil
	}
	pbMsg, err := message.Unmarshal(data[xpacket.HeaderSize:])
	if err != nil {
		ColorPrintf(Red, "can not unmarshal:%v, 0x%X err:%v\n", header.MessageID, header.MessageID, err)
		return nil, err
	}
	return &xpacket.Packet{
		Header:    header,
		PBMessage: pbMsg,
		IMessage:  message,
	}, nil
}

func (p *Client) OnPacket(remote xnetcommon.IRemote, packet xpacket.IPacket) error {
	switch pkt := packet.(type) {
	case *xpacket.Packet:
		return p.handleProtoPacket(pkt)
	case *xpacket.PacketPassThrough:
		ColorPrintf(Red, "recv unknown messageID=0x%x resultID=%d len=%d\n", pkt.Header.MessageID, pkt.Header.ResultID, len(pkt.RawData))
		if pkt.Header.ResultID != 0 {
			ColorPrintf(Yellow, "ResultError: %s\n", formatResultError(pkt.Header.ResultID))
		}
		return nil
	default:
		return xerror.Mismatch
	}
}

func (p *Client) handleProtoPacket(packet *xpacket.Packet) error {
	if _, ok := p.ignoreMsgID[packet.Header.MessageID]; ok {
		return nil
	}
	msgName := reflect.TypeOf(packet.PBMessage).Elem().Name()
	color := Green
	if packet.Header.ResultID != 0 {
		color = Yellow
	}
	fmt.Println()
	ColorPrintf(color, "MessageName: %s\n", msgName)
	headerJson, err := json.MarshalIndent(marshalHeaderMap(packet.Header), "", "  ")
	if err != nil {
		ColorPrintf(Red, "json marshal header failed: %v\n", err)
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
		log.Infof("\n======recv message======\n%s\nHeader: %s\nResultError: %s\nMessage: %s", msgName, headerJson, resultError, body)
	} else {
		log.Infof("\n======recv message======\n%s\nHeader: %s\nMessage: %s", msgName, headerJson, body)
	}

	switch msg := packet.PBMessage.(type) {
	case *pb.UserVerifyRes:
		if packet.Header.ResultID == 0 {
			p.verified = true
			p.SendHeartBeat()
		}
	case *pb.UserHeartbeatRes:
		p.nextSession = msg.GetNextSession()
	}
	return nil
}

func (p *Client) OnDisconnect(remote xnetcommon.IRemote) error {
	ColorPrintf(Red, "OnDisconnect remote=%p reason=%d\n", remote, remote.GetDisconnectReason())
	log.Warnf("OnDisconnect remote=%p reason=%d", remote, remote.GetDisconnectReason())
	p.Close()
	return nil
}
