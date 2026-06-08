package common

import (
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

const (
	OnlineServerName  = "online"
	OnlinePackageName = "online"
	OnlineServiceName = "OnlineService"

	GatewayServerName  = "gateway"
	GatewayPackageName = "gateway"
	GatewayServiceName = "GatewayService"

	CacheServerName  = "cache"
	CachePackageName = "cache"
	CacheServiceName = "CacheService"

	LoginServerName = "login"

	ConnectTicketSecretDefault = "server-dev-connect-ticket-secret"
)

const (
	// [10000,20000] 留给业务使用
	DisconnectReason_xxx xnetcommon.DisconnectReason = 10000 // 未知原因
)

func init() {
	xpacket.SetEndianMode(xpacket.LittleEndian)
}
