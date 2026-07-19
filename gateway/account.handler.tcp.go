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
// AccountHandlerTCP：处理来自客户端的 TCP 事件
// ─────────────────────────────────────────────────────────────────────────────

var GAccountHandlerTCP = &AccountHandlerTCP{}

type AccountHandlerTCP struct{}

// OnConnect 当客户端 TCP 建立成功：登记 Account 并启动「未校验超时」定时器。
func (p *AccountHandlerTCP) OnConnect(remote xnetcommon.IRemote) error {
	xlog.GLog.Infof("Client connected from: %p %s", remote, remote.GetIP())
	GAccountMgr.Add(remote)
	return nil
}

// OnCheckPacketLength 检查包长度
func (p *AccountHandlerTCP) OnCheckPacketLength(length uint32) error {
	if length < xpacket.HeaderSize || length > *xconfig.GConfigMgr.Base.PacketLengthMax {
		return xerror.Length
	}
	return nil
}

// OnCheckPacketLimit 限流校验
func (p *AccountHandlerTCP) OnCheckPacketLimit(remote xnetcommon.IRemote) error {
	_ = remote
	return nil
}

// OnUnmarshalPacket 统一反序列化（切出 Header + Body，不在网关解析业务结构）
func (p *AccountHandlerTCP) OnUnmarshalPacket(remote xnetcommon.IRemote, data []byte) (xpacket.IPacket, error) {
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
func (p *AccountHandlerTCP) OnPacket(remote xnetcommon.IRemote, packet xpacket.IPacket) error {
	pt, ok := packet.(*xpacket.PacketPassThrough)
	if !ok {
		return nil
	}

	header := pt.Header
	body := pt.RawData[xpacket.HeaderSize:header.Length]

	xlog.GLog.Debugf("phase=tcp_packet messageID=%d length=%d key=%d", header.MessageID, header.Length, header.Key)

	if header.MessageID == uint32(pb.MsgID_AccountVerifyReq_CMD) { // 登录鉴权
		err := handleAccountVerifyReq(remote, header, body)
		if err != nil {
			xlog.GLog.Warnf("handleAccountVerifyReq error: %v", err)
		}
		return err
	}

	u := GAccountMgr.Get(remote)
	if u == nil {
		xlog.GLog.Errorf("packet from unknown remote=%p messageID=%d", remote, header.MessageID)
		return nil
	}

	u.PostClientPacket(header, body)
	return nil
}

// OnDisconnect 当客户端连接断开：从 AccountMgr 摘除并清理定时器。
func (p *AccountHandlerTCP) OnDisconnect(remote xnetcommon.IRemote) error {
	u, err := GAccountMgr.Remove(remote)
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
