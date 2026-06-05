package main

import (
	"context"
	"net/http"
	pb "server/proto/pb"
)

// tokenReq 是 /api/login/token 和 /api/login/session 共用的账号 token 请求体。
type tokenReq struct {
	Account string `json:"account"` // 登录账号，去除首尾空格后不能为空
	Token   string `json:"token"`   // 一次性登录 token，不能为空
}

// tokenRes 是外部程序写入账号 token 后的响应体，不返回 uid。
type tokenRes struct {
	Account      string `json:"account"`      // 已写入 token 的账号
	Token        string `json:"token"`        // 已写入的账号 token
	ExpireSecond uint64 `json:"expireSecond"` // token 有效秒数
}

// handleLoginToken 供外部程序写入账号 token；这里只写 token，不创建 uid，也不分配 gateway。
func handleLoginToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, ok := decodeTokenReq(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), GCfgCustomCacheRPCTimeout)
	defer cancel()

	_, err := pb.GXCacheServiceService.CacheSetAccountVerifyToken(ctx, &pb.CacheSetAccountVerifyTokenReq{
		Account:      req.Account,
		Token:        req.Token,
		ExpireSecond: GCfgCustomTokenExpireSecond,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}

	writeJSON(w, http.StatusOK, &tokenRes{
		Account:      req.Account,
		Token:        req.Token,
		ExpireSecond: GCfgCustomTokenExpireSecond,
	})
}
