package main

import (
	"time"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xutil "github.com/75912001/xlib/util"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func (p *User) OnClientPacket(header *xpacket.Header, body []byte) error {
	if !p.remote.IsConnect() {
		return nil
	}
	if header.MessageID == uint32(pb.MsgIDUser_UserHeartbeatReq_CMD) {
		return p.OnHeartbeatReq(header, body)
	}
	if p.online == nil {
		xlog.GLog.Warnf("packet before verify, remote=%p messageID=%d", p.remote, header.MessageID)
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}
	if header.MessageID == uint32(pb.MsgIDUser_UserOfflineReq_CMD) {
		p.Disconnect(xnetcommon.DisconnectReasonClientShutdown)
		return nil
	}

	frame := &pb.OnlineTunnelFrame{
		Uid: p.uid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: header.MessageID,
				SessionId: header.SessionID,
				ResultId:  header.ResultID,
				Key:       p.uid,
				Body:      body,
			},
		},
	}
	if err := p.online.Send(&pb.OnlineStreamTunnelReq{Frames: []*pb.OnlineTunnelFrame{frame}}); err != nil {
		xlog.GLog.Errorf("stream send failed for online[%s]: %v", p.online.ID, err)
		return err
	}
	xlog.GLog.Infof("Message %d forwarded to online[%s]", header.MessageID, p.online.ID)
	return nil
}

// OnHeartbeatReq 处理客户端心跳请求。
//
//	验证 last_session 与上一次下发的 session 是否一致（首次允许 0）；
//	若不一致视为重放/篡改，主动断开；
//	否则生成新 session 并下发，重置心跳超时定时器。
func (p *User) OnHeartbeatReq(header *xpacket.Header, body []byte) error {
	if !p.remote.IsConnect() {
		return nil
	}
	if p.online == nil {
		xlog.GLog.Warnf("heartbeat before verify, remote=%s", p.ip)
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}

	var req pb.UserHeartbeatReq
	if err := proto.Unmarshal(body, &req); err != nil {
		xlog.GLog.Warnf("heartbeat req unmarshal failed err=%v", err)
		return errors.WithMessage(err, "UserHeartbeatReq unmarshal")
	}

	if p.hb.WaitID != 0 && req.GetLastSession() != p.hb.WaitID {
		xlog.GLog.Warnf("user[uid=%d] heartbeat session mismatch: got=%d expect=%d",
			p.uid, req.GetLastSession(), p.hb.WaitID)
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}

	next := xutil.RandomUint32()
	p.hb.WaitID = next

	p.startHeartbeatTimer()

	return p.sendHeartbeatRes(header, next)
}

func (p *User) sendHeartbeatRes(header *xpacket.Header, next uint32) error {
	return p.remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDUser_UserHeartbeatRes_CMD),
			SessionID: header.SessionID,
			ResultID:  0,
			Key:       header.Key,
		},
		PBMessage: &pb.UserHeartbeatRes{
			ServerTime:  uint64(time.Now().UnixMilli()),
			NextSession: next,
		},
	})
}
