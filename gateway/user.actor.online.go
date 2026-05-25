package main

import (
	"fmt"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

func (p *User) handleOnlineFrame(frame *pb.OnlineTunnelFrame) {
	if !p.remote.IsConnect() {
		return
	}
	uid := frame.GetUid()
	if uid != p.uid {
		xlog.PrintfErr("user actor uid mismatch: actor uid=%d frame uid=%d", p.uid, uid)
		return
	}

	switch payload := frame.Payload.(type) {
	case *pb.OnlineTunnelFrame_KickUserReq:
		xlog.PrintInfo(fmt.Sprintf("kick uid=%d reason=%d msg=%s",
			uid, payload.KickUserReq.GetReason(), payload.KickUserReq.GetMsg()))
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
	case *pb.OnlineTunnelFrame_ClientPacket:
		pkt := payload.ClientPacket
		if pkt == nil {
			return
		}
		if err := p.remote.Send(buildClientPacketPassThrough(pkt)); err != nil {
			xlog.PrintfErr("user downstream send failed uid=%d messageID=%d err=%v",
				uid, pkt.GetMessageId(), err)
		}
	default:
		xlog.PrintfErr("unexpected frame payload type for uid=%d", uid)
	}
}

func buildClientPacketPassThrough(pkt *pb.OnlineClientPacket) *xpacket.PacketPassThrough {
	header := &xpacket.Header{
		Length:    xpacket.HeaderSize + uint32(len(pkt.GetBody())),
		MessageID: pkt.GetMessageId(),
		SessionID: pkt.GetSessionId(),
		ResultID:  pkt.GetResultId(),
		Key:       pkt.GetKey(),
	}
	data := header.Pack()
	copy(data[xpacket.HeaderSize:], pkt.GetBody())
	return &xpacket.PacketPassThrough{
		Header:  header,
		RawData: data,
	}
}
