package main

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	pb "server/proto/pb"

	xconfig "github.com/75912001/xlib/config"
	xerror "github.com/75912001/xlib/error"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

// ─────────────────────────────────────────────────────────────────────────────
// UserHandlerTCP：处理来自客户端的 TCP 事件
// ─────────────────────────────────────────────────────────────────────────────

var GUserHandlerTCP = &UserHandlerTCP{}

type UserHandlerTCP struct{}

// OnConnect 当客户端 TCP 建立成功：登记 User 并启动「未校验超时」定时器。
func (h *UserHandlerTCP) OnConnect(remote xnetcommon.IRemote) error {
	xlog.PrintInfo(fmt.Sprintf("Client connected from: %p %s", remote, remote.GetIP()))
	GUserMgr.Add(remote)
	return nil
}

// OnCheckPacketLength 检查包长度
func (h *UserHandlerTCP) OnCheckPacketLength(length uint32) error {
	if length < xpacket.HeaderSize || length > *xconfig.GConfigMgr.Base.PacketLengthMax {
		return xerror.Length
	}
	return nil
}

// OnCheckPacketLimit 限流校验
func (h *UserHandlerTCP) OnCheckPacketLimit(remote xnetcommon.IRemote) error {
	_ = remote
	return nil
}

// OnUnmarshalPacket 统一反序列化（切出 Header + Body，不在网关解析业务结构）
func (h *UserHandlerTCP) OnUnmarshalPacket(remote xnetcommon.IRemote, data []byte) (xpacket.IPacket, error) {
	_ = remote
	header := xpacket.NewHeader()
	header.Unpack(data[:xpacket.HeaderSize])
	body := data[xpacket.HeaderSize:header.Length]
	return &xpacket.PacketPassThrough{
		Header:  header,
		RawData: body,
	}, nil
}

// OnPacket 报文处理核心分流器
func (h *UserHandlerTCP) OnPacket(remote xnetcommon.IRemote, packet xpacket.IPacket) error {
	pt, ok := packet.(*xpacket.PacketPassThrough)
	if !ok {
		return nil
	}

	header := pt.Header
	body := pt.RawData

	xlog.PrintInfo(fmt.Sprintf("OnPacket MessageID: %d, Length: %d, Key: %d", header.MessageID, header.Length, header.Key))

	// UserVerifyReq：登录鉴权，走 Unary gRPC（selector.Sel 内部按 uid 路由，无需预选 online）
	if header.MessageID == uint32(pb.MsgIDUser_UserVerifyReq_CMD) {
		return unaryOnlineUserOnline(remote, header, body)
	}

	u := GUserMgr.Get(remote)
	if u == nil {
		xlog.PrintfErr("packet from unknown remote=%p messageID=%d", remote, header.MessageID)
		return nil
	}

	// 心跳：网关本地处理（校验 session + 刷新心跳定时器），不下发到 online
	if header.MessageID == uint32(pb.MsgIDUser_UserHeartbeatReq_CMD) {
		u.shard.PostHeartbeat(u, header, body)
		return nil
	}

	if !u.IsVerified() {
		xlog.PrintfErr("packet before verify, remote=%p messageID=%d", remote, header.MessageID)
		u.Disconnect(xnetcommon.DisconnectReasonClientLogic)
		return nil
	}

	uid := u.uid.Load()

	// 其余消息：使用登录时绑定的 online 实例，经双向 StreamTunnel 透传
	online := u.GetOnline()
	if online == nil {
		xlog.PrintfErr("online not bound for uid=%d", uid)
		u.Disconnect(xnetcommon.DisconnectReasonServerShutdown)
		return nil
	}

	frame := &pb.OnlineTunnelFrame{Uid: uid}

	// UserOfflineReq：客户端主动离线信令
	if header.MessageID == uint32(pb.MsgIDUser_UserOfflineReq_CMD) {
		var offlineReq pb.UserOfflineReq
		if err := proto.Unmarshal(body, &offlineReq); err != nil {
			xlog.PrintfErr("UserOfflineReq unmarshal failed uid=%d: %v", uid, err)
			return err
		}
		frame.Payload = &pb.OnlineTunnelFrame_UserOfflineReq{
			UserOfflineReq: &pb.OnlineUserOfflineReq{
				Reason: offlineReq.GetReason(),
			},
		}
	} else {
		frame.Payload = &pb.OnlineTunnelFrame_ClientPacket{
			ClientPacket: &pb.OnlineClientPacket{
				MessageId: header.MessageID,
				SessionId: header.SessionID,
				ResultId:  header.ResultID,
				Key:       uid,
				Body:      body,
			},
		}
	}

	if err := online.Send(&pb.OnlineStreamTunnelReq{Frames: []*pb.OnlineTunnelFrame{frame}}); err != nil {
		xlog.PrintfErr("stream send failed for online[%s]: %v", online.ID, err)
		return err
	}

	xlog.PrintInfo(fmt.Sprintf("Message %d forwarded to online[%s]", header.MessageID, online.ID))
	return nil
}

// OnDisconnect 当客户端连接断开：从 UserMgr 摘除并清理定时器。
func (h *UserHandlerTCP) OnDisconnect(remote xnetcommon.IRemote) error {
	u := GUserMgr.Remove(remote)
	if u == nil {
		xlog.PrintInfo(fmt.Sprintf("Client disconnected: %p reason=%d", remote, remote.GetDisconnectReason()))
		return nil
	}
	xlog.PrintInfo(fmt.Sprintf("Client disconnected: %p %s reason=%d", remote, u.ip, remote.GetDisconnectReason()))
	return nil
}
