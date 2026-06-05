package main

import (
	"encoding/json"
	"io"
	"net/http"
	"server/common"
	"strings"

	xlog "github.com/75912001/xlib/log"
)

// newHTTPServer 创建 login HTTP 服务，并按配置注册 token/session 两个接口。
func newHTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(GCfgCustomTokenPath, handleLoginToken)
	mux.HandleFunc(GCfgCustomSessionPath, handleLoginSession)
	return &http.Server{
		Addr:              GCfgCustomHTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: GCfgCustomReadHeaderTimeout,
	}
}

// decodeTokenReq 读取并校验账号 token 请求，拒绝未知字段和空 account/token。
func decodeTokenReq(w http.ResponseWriter, r *http.Request) (*tokenReq, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, GCfgCustomMaxBodyBytes)
	defer r.Body.Close()

	var req tokenReq
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request")
		return nil, false
	}

	req.Account = strings.TrimSpace(req.Account)
	if req.Account == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid account or token")
		return nil, false
	}
	return &req, true
}

// cacheErrorToHTTP 将 Cache gRPC 错误转换为 login HTTP 错误码和错误信息。
func cacheErrorToHTTP(err error) (int, string) {
	return common.GRPCStatusToHTTP(err, "cache not available")
}

// writeError 写入统一 JSON 错误响应。
func writeError(w http.ResponseWriter, statusCode int, message string) {
	// errorRes 是 login HTTP 接口统一错误响应体。
	type errorRes struct {
		Error string `json:"error"` // 错误信息
	}
	writeJSON(w, statusCode, &errorRes{Error: message})
}

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		xlog.GLog.Warnf("write http response failed: %v", err)
	}
}
