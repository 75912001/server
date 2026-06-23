package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"server/common"
	"strings"

	xconfig "github.com/75912001/xlib/config"
	xlog "github.com/75912001/xlib/log"
	"gopkg.in/yaml.v3"
)

// newHTTPServer 创建 login HTTP 服务, 并按配置注册 accountVerifyToken/session/emailSession 三个接口。
func newHTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(GCfgCustomAccountVerifyTokenPath, handleLoginAccountVerifyToken)
	mux.HandleFunc(GCfgCustomSessionPath, handleLoginSession)
	mux.HandleFunc(GCfgCustomEmailSessionPath, handleLoginEmailSession)
	return &http.Server{
		Addr:              GCfgCustomHTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: GCfgCustomReadHeaderTimeout,
	}
}

// decodeAccountVerifyTokenReq 读取并校验 accountVerifyToken 请求, 拒绝未知字段和空 account/accountVerifyToken。
func decodeAccountVerifyTokenReq(w http.ResponseWriter, r *http.Request) (*accountVerifyTokenReq, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, GCfgCustomMaxBodyBytes)
	defer r.Body.Close()

	var req accountVerifyTokenReq
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
	if req.Account == "" || req.AccountVerifyToken == "" {
		writeError(w, http.StatusBadRequest, "invalid account or accountVerifyToken")
		return nil, false
	}
	return &req, true
}

// decodeEmailSessionReq 读取并校验 email/password 登录请求, email 会 trim 后转小写, password 保持原始值精确匹配。
func decodeEmailSessionReq(w http.ResponseWriter, r *http.Request) (*emailSessionReq, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, GCfgCustomMaxBodyBytes)
	defer r.Body.Close()

	var req emailSessionReq
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

	req.Email = normalizeEmail(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid email or password")
		return nil, false
	}
	return &req, true
}

// normalizeEmail 统一 email 账号格式, 避免大小写造成同一邮箱生成多个 account。
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// loadEmailPasswordUsers 每次从当前运行配置文件读取 email/password 账号表。
func loadEmailPasswordUsers() (map[string]string, error) {
	configPath := xconfig.GConfigMgr.ExecutablePath
	if configPath == "" {
		return nil, errLoginCredentialConfigInvalid
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg emailPasswordConfig
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	users := make(map[string]string, len(cfg.Custom.EmailPasswordUsers))
	for _, user := range cfg.Custom.EmailPasswordUsers {
		email := normalizeEmail(user.Email)
		if email == "" || user.Password == "" {
			return nil, errLoginCredentialConfigInvalid
		}
		if _, exists := users[email]; exists {
			return nil, errLoginCredentialConfigInvalid
		}
		users[email] = user.Password
	}
	return users, nil
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
