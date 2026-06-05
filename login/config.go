package main

import (
	"time"

	"server/common"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomHTTPAddr string                 // HTTP 监听地址
var GCfgCustomTokenPath string                // 外部程序写账号 token 的 HTTP 路径
var GCfgCustomSessionPath string              // 客户端换取 gateway 登录信息的 HTTP 路径
var GCfgCustomTokenExpireSecond uint64        // 账号 token 有效秒数
var GCfgCustomTicketExpireSecond uint64       // connectTicket 有效秒数
var GCfgCustomTicketSecret string             // connectTicket HMAC 签名密钥
var GCfgCustomReadHeaderTimeout time.Duration // HTTP 读取请求头超时时间
var GCfgCustomShutdownTimeout time.Duration   // HTTP 优雅关闭等待时间
var GCfgCustomCacheRPCTimeout time.Duration   // 调用 Cache gRPC 超时时间
var GCfgCustomMaxBodyBytes int64              // HTTP 请求体最大字节数

// initCustomConfig 从 xlib 配置管理器读取 login 自定义配置。
func initCustomConfig() {
	GCfgCustomHTTPAddr = xconfig.GConfigMgr.GetCustomString("httpAddr")
	GCfgCustomTokenPath = xconfig.GConfigMgr.GetCustomString("tokenPath", "/api/login/token")
	GCfgCustomSessionPath = xconfig.GConfigMgr.GetCustomString("sessionPath", "/api/login/session")
	GCfgCustomTokenExpireSecond = uint64(xconfig.GConfigMgr.GetCustomDuration("tokenExpireSecond", 10*time.Second) / time.Second)
	GCfgCustomTicketExpireSecond = uint64(xconfig.GConfigMgr.GetCustomDuration("ticketExpireSecond", 10*time.Second) / time.Second)
	GCfgCustomTicketSecret = xconfig.GConfigMgr.GetCustomString("ticketSecret", common.ConnectTicketSecretDefault)
	GCfgCustomReadHeaderTimeout = xconfig.GConfigMgr.GetCustomDuration("readHeaderTimeout", 5*time.Second)
	GCfgCustomShutdownTimeout = xconfig.GConfigMgr.GetCustomDuration("shutdownTimeout", 10*time.Second)
	GCfgCustomCacheRPCTimeout = xconfig.GConfigMgr.GetCustomDuration("cacheRPCTimeout", 3*time.Second)
	GCfgCustomMaxBodyBytes = xconfig.GConfigMgr.GetCustomInt64("maxBodyBytes", 4096)
}
