package main

import (
	"fmt"

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
func (p *UserHandlerTCP) OnConnect(remote xnetcommon.IRemote) error {
	xlog.PrintInfo(fmt.Sprintf("Client connected from: %p %s", remote, remote.GetIP()))
	GUserMgr.Add(remote)
	return nil
}

// OnCheckPacketLength 检查包长度
func (p *UserHandlerTCP) OnCheckPacketLength(length uint32) error {
	if length < xpacket.HeaderSize || length > *xconfig.GConfigMgr.Base.PacketLengthMax {
		return xerror.Length
	}
	return nil
}

// OnCheckPacketLimit 限流校验
func (p *UserHandlerTCP) OnCheckPacketLimit(remote xnetcommon.IRemote) error {
	_ = remote
	return nil
}

// OnUnmarshalPacket 统一反序列化（切出 Header + Body，不在网关解析业务结构）
func (p *UserHandlerTCP) OnUnmarshalPacket(remote xnetcommon.IRemote, data []byte) (xpacket.IPacket, error) {
	_ = remote
	header := xpacket.NewHeader()
	header.Unpack(data[:xpacket.HeaderSize])
	return &xpacket.PacketPassThrough{
		Header:  header,
		RawData: data,
	}, nil
}

// OnPacket 报文处理核心分流器
func (p *UserHandlerTCP) OnPacket(remote xnetcommon.IRemote, packet xpacket.IPacket) error {
	pt, ok := packet.(*xpacket.PacketPassThrough)
	if !ok {
		return nil
	}

	header := pt.Header
	body := pt.RawData[xpacket.HeaderSize:header.Length]

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

	u.PostClientPacket(header, body)
	return nil
}

// OnDisconnect 当客户端连接断开：从 UserMgr 摘除并清理定时器。
func (p *UserHandlerTCP) OnDisconnect(remote xnetcommon.IRemote) error {
	u := GUserMgr.Remove(remote)
	if u == nil {
		xlog.PrintInfo(fmt.Sprintf("Client disconnected: %p reason=%d", remote, remote.GetDisconnectReason()))
		return nil
	}
	xlog.PrintInfo(fmt.Sprintf("Client disconnected: %p %s reason=%d", remote, u.ip, remote.GetDisconnectReason()))
	return nil
}
