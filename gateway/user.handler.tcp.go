package main

import (
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
	xlog.GLog.Infof("Client connected from: %p %s", remote, remote.GetIP())
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

	rawData := append([]byte(nil), data...)
	return &xpacket.PacketPassThrough{
		Header:  header,
		RawData: rawData,
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

	xlog.GLog.Debugf("phase=tcp_packet messageID=%d length=%d key=%d", header.MessageID, header.Length, header.Key)

	if header.MessageID == uint32(pb.MsgIDUser_UserVerifyReq_CMD) { // 登录鉴权
		err := handleUserVerifyReq(remote, header, body)
		if err != nil {
			xlog.GLog.Warnf("handleUserVerifyReq error: %v", err)
		}
		return err
	}

	u := GUserMgr.Get(remote)
	if u == nil {
		xlog.GLog.Errorf("packet from unknown remote=%p messageID=%d", remote, header.MessageID)
		return nil
	}

	u.PostClientPacket(header, body)
	return nil
}

// OnDisconnect 当客户端连接断开：从 UserMgr 摘除并清理定时器。
func (p *UserHandlerTCP) OnDisconnect(remote xnetcommon.IRemote) error {
	u, err := GUserMgr.Remove(remote)
	if err != nil {
		xlog.GLog.Warnf("Client cleanup failed: %p reason=%d err=%v", remote, remote.GetDisconnectReason(), err)
	}
	if u == nil {
		xlog.GLog.Infof("Client disconnected: %p reason=%d", remote, remote.GetDisconnectReason())
		return nil
	}
	xlog.GLog.Infof("Client disconnected: %p %s reason=%d", remote, u.ip, remote.GetDisconnectReason())
	return nil
}
