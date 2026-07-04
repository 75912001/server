package main

import (
	"context"
	"net/http"
	"time"

	"server/common"
	pb "server/proto/pb"
)

// sessionRes 是客户端换取 gateway 登录信息后的响应体。
type sessionRes struct {
	Account                 string `json:"account"`                 // 登录账号
	Aid                     uint64 `json:"aid"`                     // Cache 根据账号解析或创建的可信 aid
	ConnectTicket           string `json:"connectTicket"`           // 客户端连接 gateway 时携带的短期登录票据
	TicketExpireTimestampMs int64  `json:"ticketExpireTimestampMs"` // connectTicket 过期时间戳，单位毫秒
	GatewayKey              string `json:"gatewayKey"`              // login 分配的目标 gateway etcd key
	GatewayAddr             string `json:"gatewayAddr"`             // 客户端连接目标 gateway 的 TCP 地址
}

// handleLoginSession 供客户端消费 accountVerifyToken, 换取 aid、connectTicket 和目标 gateway。
func handleLoginSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, ok := decodeAccountVerifyTokenReq(w, r)
	if !ok {
		return
	}

	cacheCtx, cacheCancel := context.WithTimeout(r.Context(), GCfgCustomCacheRPCTimeout)
	defer cacheCancel()

	cacheRes, err := pb.GXCacheServiceService.CacheUseAccountVerifyToken(cacheCtx, &pb.CacheUseAccountVerifyTokenReq{
		Account:            req.Account,
		AccountVerifyToken: req.AccountVerifyToken,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}
	aid := cacheRes.GetAid()
	writeLoginSession(w, req.Account, aid)
}

// writeLoginSession 根据可信 aid 选择 gateway, 签发 connectTicket 并写入 session 响应。
func writeLoginSession(w http.ResponseWriter, account string, aid uint64) {
	if aid == 0 {
		writeError(w, http.StatusBadGateway, "cache account aid is empty")
		return
	}

	gateway, ok := GGatewayMgr.ReserveByAvailableLoad()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "gateway not available")
		return
	}

	now := time.Now()
	payload, err := common.NewConnectTicketPayload(
		aid,
		account,
		gateway.Key,
		time.Duration(GCfgCustomTicketExpireSecond)*time.Second,
		now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "new connect ticket failed")
		return
	}
	connectTicket, err := common.SignConnectTicket(payload, GCfgCustomTicketSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sign connect ticket failed")
		return
	}

	writeJSON(w, http.StatusOK, &sessionRes{
		Account:                 account,
		Aid:                     aid,
		ConnectTicket:           connectTicket,
		TicketExpireTimestampMs: payload.ExpireTimestampMs,
		GatewayKey:              gateway.Key,
		GatewayAddr:             gateway.Addr,
	})
}
