package main

import (
	"context"
	"net/http"
	pb "server/proto/pb"
)

// accountVerifyTokenReq 是 accountVerifyToken 写入和 session 消费共用的请求体。
type accountVerifyTokenReq struct {
	Account            string `json:"account"`            // 登录账号, 去除首尾空格后不能为空
	AccountVerifyToken string `json:"accountVerifyToken"` // accountVerifyToken, 不能为空
}

// accountVerifyTokenRes 是外部程序写入 accountVerifyToken 后的响应体, 不返回 uid。
type accountVerifyTokenRes struct {
	Account            string `json:"account"`            // 已写入 accountVerifyToken 的账号
	AccountVerifyToken string `json:"accountVerifyToken"` // 已写入的 accountVerifyToken
	ExpireSecond       uint64 `json:"expireSecond"`       // accountVerifyToken 有效秒数
}

// handleLoginAccountVerifyToken 供外部程序写入 accountVerifyToken; 这里只写凭证, 不创建 uid, 也不分配 gateway。
func handleLoginAccountVerifyToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, ok := decodeAccountVerifyTokenReq(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), GCfgCustomCacheRPCTimeout)
	defer cancel()

	_, err := pb.GXCacheServiceService.CacheSetAccountVerifyToken(ctx, &pb.CacheSetAccountVerifyTokenReq{
		Account:            req.Account,
		AccountVerifyToken: req.AccountVerifyToken,
		ExpireSecond:       GCfgCustomAccountVerifyTokenExpireSecond,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}

	writeJSON(w, http.StatusOK, &accountVerifyTokenRes{
		Account:            req.Account,
		AccountVerifyToken: req.AccountVerifyToken,
		ExpireSecond:       GCfgCustomAccountVerifyTokenExpireSecond,
	})
}
