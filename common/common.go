package common

import (
	xnetcommon "github.com/75912001/xlib/net/common"
)

const (
	OnlineServerName  = "online"
	OnlinePackageName = "online"
	OnlineServiceName = "OnlineService"

	GatewayServerName  = "gateway"
	GatewayPackageName = "gateway"
	GatewayServiceName = "GatewayService"
)

const (
	// [10000,20000] 留给业务使用
	DisconnectReason_xxx xnetcommon.DisconnectReason = 10000 // 未知原因
)
