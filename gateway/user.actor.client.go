package main

import (
	"server/common"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
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
//	验证 last_gateway_session 与 gateway 本地 gatewaySession 是否一致；
//	若不一致视为重放/篡改，主动断开；
//	否则生成新 gatewaySession，同步 online/cache 后下发，并重置心跳超时定时器。
func (p *User) OnHeartbeatReq(header *xpacket.Header, body []byte) error {
	var req pb.UserHeartbeatReq
	if err := proto.Unmarshal(body, &req); err != nil {
		return errors.WithMessagef(err, "UserHeartbeatReq unmarshal %v", xruntime.Location())
	}

	lastGatewaySession := req.GetLastGatewaySession()
	if lastGatewaySession == "" || lastGatewaySession != p.gatewaySession {
		p.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return errors.WithMessagef(xerror.Mismatch, "heartbeat gatewaySession mismatch for user[uid=%d] got=%s expect=%s %v",
			p.uid, lastGatewaySession, p.gatewaySession, xruntime.Location())
	}

	nextGatewaySession, err := common.NewRandomGatewaySession()
	if err != nil {
		p.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return errors.WithMessagef(err, "new random gatewaySession for user[uid=%d] %v", p.uid, xruntime.Location())
	}

	if err := p.UpdateGatewaySession(nextGatewaySession); err != nil {
		return errors.WithMessagef(err, "update gatewaySession for user[uid=%d] %v", p.uid, xruntime.Location())
	}

	p.restartHeartbeatTimer()

	return sendClientRes(p.remote,
		uint32(pb.MsgIDUser_UserHeartbeatRes_CMD),
		header.SessionID,
		xerror.Success.Code(),
		header.Key,
		&pb.UserHeartbeatRes{
			ServerTime:         time.Now().UnixMilli(),
			NextGatewaySession: nextGatewaySession,
		},
	)
}
