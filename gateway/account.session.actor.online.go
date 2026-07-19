package main

import (
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xpacket "github.com/75912001/xlib/packet"
)

func (p *Account) handleOnlineFrame(frame *pb.OnlineTunnelFrame) {
	if !p.remote.IsConnect() {
		xlog.GLog.Warnf("remote is not connect %v", p.remote)
		return
	}
	aid := frame.GetAid()
	if aid != p.aid {
		xlog.GLog.Warnf("account actor aid mismatch: actor aid:%d frame aid:%d", p.aid, aid)
		return
	}

	switch payload := frame.Payload.(type) {
	case *pb.OnlineTunnelFrame_ClientPacket:
		pkt := payload.ClientPacket
		if pkt == nil {
			return
		}
		if err := p.remote.Send(buildClientPacketPassThrough(pkt)); err != nil {
			xlog.GLog.Errorf("account downstream send failed aid:%d messageID:%d err:%v",
				aid, pkt.GetMessageId(), err)
		}
	default:
		xlog.GLog.Errorf("unexpected frame payload type for aid:%d", aid)
	}
}

func buildClientPacketPassThrough(pkt *pb.OnlineClientPacket) *xpacket.PacketPassThrough {
	header := &xpacket.Header{
		Length:    xpacket.HeaderSize + uint32(len(pkt.GetBody())),
		MessageID: pkt.GetMessageId(),
		SessionID: 0,
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
