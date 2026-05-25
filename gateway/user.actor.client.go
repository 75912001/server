package main

import (
	"fmt"
	"time"

	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xutil "github.com/75912001/xlib/util"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func (u *User) OnClientPacket(header *xpacket.Header, body []byte) error {
	if !u.remote.IsConnect() {
		return nil
	}
	if header.MessageID == uint32(pb.MsgIDUser_UserHeartbeatReq_CMD) {
		return u.OnHeartbeatReq(header, body)
	}
	if u.online == nil {
		xlog.PrintfErr("packet before verify, remote=%p messageID=%d", u.remote, header.MessageID)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}
	if header.MessageID == uint32(pb.MsgIDUser_UserOfflineReq_CMD) {
		u.Disconnect(xnetcommon.DisconnectReasonClientShutdown)
		return nil
	}

	frame := &pb.OnlineTunnelFrame{
		Uid: u.uid,
		Payload: &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: header.MessageID,
				SessionId: header.SessionID,
				ResultId:  header.ResultID,
				Key:       u.uid,
				Body:      body,
			},
		},
	}
	if err := u.online.Send(&pb.OnlineStreamTunnelReq{Frames: []*pb.OnlineTunnelFrame{frame}}); err != nil {
		xlog.PrintfErr("stream send failed for online[%s]: %v", u.online.ID, err)
		return err
	}
	xlog.PrintInfo(fmt.Sprintf("Message %d forwarded to online[%s]", header.MessageID, u.online.ID))
	return nil
}

// OnHeartbeatReq 处理客户端心跳请求。
//
//	验证 last_session 与上一次下发的 session 是否一致（首次允许 0）；
//	若不一致视为重放/篡改，主动断开；
//	否则生成新 session 并下发，重置心跳超时定时器。
func (u *User) OnHeartbeatReq(header *xpacket.Header, body []byte) error {
	if !u.remote.IsConnect() {
		return nil
	}
	if u.online == nil {
		xlog.PrintfErr("heartbeat before verify, remote=%s", u.ip)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}

	var req pb.UserHeartbeatReq
	if err := proto.Unmarshal(body, &req); err != nil {
		return errors.WithMessage(err, "UserHeartbeatReq unmarshal")
	}

	if u.hb.WaitID != 0 && req.GetLastSession() != u.hb.WaitID {
		xlog.PrintfErr("user[uid=%d] heartbeat session mismatch: got=%d expect=%d",
			u.uid, req.GetLastSession(), u.hb.WaitID)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return errors.New("heartbeat session mismatch")
	}

	next := xutil.RandomUint32()
	u.hb.WaitID = next

	u.startHeartbeatTimer()

	return u.remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDUser_UserHeartbeatRes_CMD),
			SessionID: header.SessionID,
			Key:       header.Key,
		},
		PBMessage: &pb.UserHeartbeatRes{
			ServerTime:  uint64(time.Now().UnixMilli()),
			NextSession: next,
		},
	})
}
