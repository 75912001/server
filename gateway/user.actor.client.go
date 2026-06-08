package main

import (
	"server/common"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	xutil "github.com/75912001/xlib/util"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func (p *User) OnClientPacket(header *xpacket.Header, body []byte) error {
	if p.IsClosed() {
		return errors.WithMessagef(xerror.Disconnect, "remote not connected: %p %v", p.remote, xruntime.Location())
	}

	if p.online == nil { // 在线实例未找到，可能是未通过验证
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return errors.WithMessagef(common.ECGatewayOnlineNotFound, "online not found for user[uid=%v] remote:%v packet messageID=%d %v",
			p.uid, p.remote, header.MessageID, xruntime.Location())
	}

	// 通过验证
	switch header.MessageID {
	case uint32(pb.MsgIDUser_UserHeartbeatReq_CMD):
		return p.OnHeartbeatReq(header, body)
	case uint32(pb.MsgIDUser_UserOfflineReq_CMD):
		p.Disconnect(xnetcommon.DisconnectReasonClientShutdown)
		return nil
	default:
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
		return errors.WithMessagef(err, "stream send failed for online %v %v", p.online.Key, xruntime.Location())
	}
	return nil
}

// OnHeartbeatReq 处理客户端心跳请求。
//
//	验证 last_heartbeat_session 与 gateway 本地 heartbeatSession 是否一致；
//	heartbeatSession 是可轮换认证凭证，userSession 是固定连接身份；
//	若不一致视为重放/篡改，主动断开；
//	一致时生成新 heartbeatSession，刷新 cache TTL 后下发，并重置心跳超时定时器。
func (p *User) OnHeartbeatReq(header *xpacket.Header, body []byte) error {
	var req pb.UserHeartbeatReq
	if err := proto.Unmarshal(body, &req); err != nil {
		return errors.WithMessagef(err, "UserHeartbeatReq unmarshal %v", xruntime.Location())
	}

	lastHeartbeatSession := req.GetLastHeartbeatSession()
	if lastHeartbeatSession == "" || lastHeartbeatSession != p.heartbeatSession {
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return errors.WithMessagef(xerror.Mismatch, "phase=heartbeat uid=%d reason=heartbeatSession_mismatch got=%s expect=%s %v",
			p.uid, lastHeartbeatSession, p.heartbeatSession, xruntime.Location())
	}

	nextHeartbeatSession, err := xutil.RandomHex32()
	if err != nil {
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return errors.WithMessagef(err, "new heartbeatSession for user[uid=%d] %v", p.uid, xruntime.Location())
	}

	if err := p.UpdateHeartbeatSession(nextHeartbeatSession); err != nil {
		return errors.WithMessagef(err, "update heartbeatSession for user[uid=%d] %v", p.uid, xruntime.Location())
	}

	p.restartHeartbeatTimer()

	return sendClientRes(p.remote,
		uint32(pb.MsgIDUser_UserHeartbeatRes_CMD),
		header.SessionID,
		xerror.Success.Code(),
		header.Key,
		&pb.UserHeartbeatRes{
			ServerTime:           time.Now().UnixMilli(),
			NextHeartbeatSession: nextHeartbeatSession,
		},
	)
}
