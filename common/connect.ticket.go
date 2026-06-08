package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	xutil "github.com/75912001/xlib/util"
)

// ConnectTicketVersion 是当前 connectTicket payload 的协议版本。
const ConnectTicketVersion = 1

var (
	// ErrConnectTicketInvalid 表示票据格式、字段或签名参数不合法。
	ErrConnectTicketInvalid = errors.New("connect ticket invalid")
	// ErrConnectTicketExpired 表示票据已经过期。
	ErrConnectTicketExpired = errors.New("connect ticket expired")
	// ErrConnectTicketKeyMismatch 表示票据绑定的 gateway 与当前 gateway 不一致。
	ErrConnectTicketKeyMismatch = errors.New("connect ticket key mismatch")
	// ErrConnectTicketUIDMismatch 表示票据绑定的 uid 与客户端提交的 uid 不一致。
	ErrConnectTicketUIDMismatch = errors.New("connect ticket uid mismatch")
	// ErrConnectTicketSignMismatch 表示 HMAC 签名校验失败。
	ErrConnectTicketSignMismatch = errors.New("connect ticket signature mismatch")
)

// ConnectTicketPayload 是 login 签发给客户端、gateway 验签后信任的登录票据内容。
type ConnectTicketPayload struct {
	Version    uint32 `json:"version"`    // 票据协议版本
	UID        uint64 `json:"uid"`        // Cache 解析账号后得到的可信 uid
	Account    string `json:"account"`    // 登录账号
	GatewayKey string `json:"gatewayKey"` // 票据绑定的目标 gateway etcd key
	Nonce      string `json:"nonce"`      // 每次签发生成的随机数，避免同用户同 gateway 票据重复
	IssuedAt   int64  `json:"issuedAt"`   // 签发时间戳，单位毫秒
	ExpireAt   int64  `json:"expireAt"`   // 过期时间戳，单位毫秒
}

// ConnectTicketVerifyOptions 是 gateway 验证 connectTicket 时的本地约束。
type ConnectTicketVerifyOptions struct {
	Secret     string    // HMAC 签名密钥，login 和 gateway 必须一致
	GatewayKey string    // 当前 gateway key，非空时要求与票据 gatewayKey 一致
	UID        uint64    // 客户端提交的 uid，非 0 时要求与票据 uid 一致
	Now        time.Time // 校验时间；为空时使用当前时间
}

// NewConnectTicketPayload 创建 connectTicket payload，不做签名。
func NewConnectTicketPayload(uid uint64, account string, gatewayKey string, ttl time.Duration, now time.Time) (*ConnectTicketPayload, error) {
	if now.IsZero() {
		now = time.Now()
	}
	nonce, err := xutil.RandomHex32()
	if err != nil {
		return nil, err
	}
	return &ConnectTicketPayload{
		Version:    ConnectTicketVersion,
		UID:        uid,
		Account:    account,
		GatewayKey: gatewayKey,
		Nonce:      nonce,
		IssuedAt:   now.UnixMilli(),
		ExpireAt:   now.Add(ttl).UnixMilli(),
	}, nil
}

// SignConnectTicket 将 payload 序列化后用 HMAC-SHA256 签名，返回 payload.signature 格式的票据。
func SignConnectTicket(payload *ConnectTicketPayload, secret string) (string, error) {
	if payload == nil || secret == "" || payload.Version != ConnectTicketVersion || payload.UID == 0 ||
		payload.Account == "" || payload.GatewayKey == "" || payload.Nonce == "" ||
		payload.IssuedAt == 0 || payload.ExpireAt == 0 {
		return "", ErrConnectTicketInvalid
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(data)
	signPart := signConnectTicketPart(payloadPart, secret)
	return payloadPart + "." + signPart, nil
}

// VerifyConnectTicket 校验 connectTicket 签名、字段、目标 gateway、uid 和过期时间。
func VerifyConnectTicket(ticket string, opts ConnectTicketVerifyOptions) (*ConnectTicketPayload, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || opts.Secret == "" {
		return nil, ErrConnectTicketInvalid
	}
	wantSign := signConnectTicketPart(parts[0], opts.Secret)
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(wantSign)) != 1 {
		return nil, ErrConnectTicketSignMismatch
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrConnectTicketInvalid
	}
	var payload ConnectTicketPayload
	if err = json.Unmarshal(data, &payload); err != nil {
		return nil, ErrConnectTicketInvalid
	}
	if payload.Version != ConnectTicketVersion || payload.UID == 0 || payload.Account == "" ||
		payload.GatewayKey == "" || payload.Nonce == "" || payload.IssuedAt == 0 ||
		payload.ExpireAt == 0 {
		return nil, ErrConnectTicketInvalid
	}
	if opts.GatewayKey != "" && payload.GatewayKey != opts.GatewayKey {
		return nil, ErrConnectTicketKeyMismatch
	}
	if opts.UID != 0 && payload.UID != opts.UID {
		return nil, ErrConnectTicketUIDMismatch
	}
	if opts.Now.UnixMilli() > payload.ExpireAt {
		return nil, ErrConnectTicketExpired
	}
	return &payload, nil
}

// signConnectTicketPart 对 base64 payload 部分做 HMAC-SHA256 签名。
func signConnectTicketPart(payloadPart string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadPart))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
