package common

import xerror "github.com/75912001/xlib/error"

// [0x10000,0x1FFFF] gateway
// [0x20000,0x2FFFF] online
// [0x30000,0x3FFFF] cache
var (
	// gateway
	ECGatewayOnlineNotFound = xerror.NewError(0x10000).WithName("ECGatewayOnlineNotFound").WithDesc("gateway-online-not-found-gateway未找到online服务")
	ECGatewayInvalidUID     = xerror.NewError(0x10001).WithName("ECGatewayInvalidUID").WithDesc("gateway-invalid-uid-uid无效")
	ECGatewayUIDNotFound    = xerror.NewError(0x10002).WithName("ECGatewayUIDNotFound").WithDesc("gateway-uid-not-found-uid未找到")

	// online

	// cache
	ECCacheInvalidArgument = xerror.NewError(0x30000).WithName("ECCacheInvalidArgument").WithDesc("cache-invalid-argument-参数无效")
	ECCacheKeyNotFound     = xerror.NewError(0x30001).WithName("ECCacheKeyNotFound").WithDesc("cache-key-not-found-未找到key")
	ECCacheRedisError      = xerror.NewError(0x30002).WithName("ECCacheRedisError").WithDesc("cache-redis-error-redis错误")
)
