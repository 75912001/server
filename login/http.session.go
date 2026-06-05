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
	Account        string `json:"account"`        // 登录账号
	Uid            uint64 `json:"uid"`            // Cache 根据账号解析或创建的可信 uid
	ConnectTicket  string `json:"connectTicket"`  // 客户端连接 gateway 时携带的短期登录票据
	TicketExpireAt int64  `json:"ticketExpireAt"` // connectTicket 过期时间戳，单位毫秒
	GatewayKey     string `json:"gatewayKey"`     // login 分配的目标 gateway etcd key
	GatewayAddr    string `json:"gatewayAddr"`    // 客户端连接目标 gateway 的 TCP 地址
}

// handleLoginSession 供客户端消费账号 token，换取 uid、connectTicket 和目标 gateway。
func handleLoginSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, ok := decodeTokenReq(w, r)
	if !ok {
		return
	}

	cacheCtx, cacheCancel := context.WithTimeout(r.Context(), GCfgCustomCacheRPCTimeout)
	defer cacheCancel()

	cacheRes, err := pb.GXCacheServiceService.CacheUseAccountVerifyToken(cacheCtx, &pb.CacheUseAccountVerifyTokenReq{
		Account: req.Account,
		Token:   req.Token,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}
	uid := cacheRes.GetUid()
	if uid == 0 {
		writeError(w, http.StatusBadGateway, "cache account uid is empty")
		return
	}

	gateway, ok := GGatewayMgr.GetByAvailableLoad()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "gateway not available")
		return
	}

	now := time.Now()
	payload, err := common.NewConnectTicketPayload(
		uid,
		req.Account,
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
		Account:        req.Account,
		Uid:            uid,
		ConnectTicket:  connectTicket,
		TicketExpireAt: payload.ExpireAt,
		GatewayKey:     gateway.Key,
		GatewayAddr:    gateway.Addr,
	})
}
