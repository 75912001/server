package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	xruntimeconstants "github.com/75912001/xlib/runtime/constants"
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

	GatewaySessionSecret = "menglc-session"
)

const (
	// [10000,20000] 留给业务使用
	DisconnectReason_xxx xnetcommon.DisconnectReason = 10000 // 未知原因
)

func init() {
	xruntime.SetRunMode(xruntimeconstants.RunModeDebug)
	xpacket.SetEndianMode(xpacket.LittleEndian)
}

func NewGatewaySession(uid uint64, gatewayKey string, gatewayNonce string) string {
	data := strconv.FormatUint(uid, 10) + ":" + gatewayKey + ":" + gatewayNonce + ":" + GatewaySessionSecret
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func NewGatewayNonce() (string, error) {
	return newRandomHex32()
}

func NewRandomGatewaySession() (string, error) {
	return newRandomHex32()
}

func newRandomHex32() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
