package common

import xerror "github.com/75912001/xlib/error"

// [0x10000, 0x1FFFF]
var (
	ECGatewayOnlineNotFound = xerror.NewError(0x10000).WithName("ECGatewayOnlineNotFound").WithDesc("gateway-online-not-found-gateway未找到online服务")
)
