package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"server/common"
	pb "server/proto/pb"

	xlog "github.com/75912001/xlib/log"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type tokenReq struct {
	Account string `json:"account"`
	Token   string `json:"token"`
}

type tokenRes struct {
	Account      string `json:"account"`
	Token        string `json:"token"`
	ExpireSecond uint64 `json:"expireSecond"`
}

type sessionRes struct {
	Account        string `json:"account"`
	Uid            uint64 `json:"uid"`
	GatewayNonce   string `json:"gatewayNonce"`
	GatewaySession string `json:"gatewaySession"`
	GatewayKey     string `json:"gatewayKey"`
	GatewayAddr    string `json:"gatewayAddr"`
	ExpireSecond   uint64 `json:"sessionExpireSecond"`
}

type errorRes struct {
	Error string `json:"error"`
}

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

	gatewayNonce, err := common.NewGatewayNonce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "new gateway nonce failed")
		return
	}
	gatewaySession := common.NewGatewaySession(uid, gateway.Key, gatewayNonce)
	gatewayCtx, gatewayCancel := context.WithTimeout(r.Context(), GCfgCustomGatewayRPCTimeout)
	defer gatewayCancel()

	err = prepareGatewayLogin(gatewayCtx, gateway, uid, req.Account, gatewayNonce, gatewaySession)
	if err != nil {
		statusCode, message := gatewayErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}

	writeJSON(w, http.StatusOK, &sessionRes{
		Account:        req.Account,
		Uid:            uid,
		GatewayNonce:   gatewayNonce,
		GatewaySession: gatewaySession,
		GatewayKey:     gateway.Key,
		GatewayAddr:    gateway.Addr,
		ExpireSecond:   GCfgCustomSessionExpireSecond,
	})
}

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

func cacheErrorToHTTP(err error) (int, string) {
	status, ok := grpcstatus.FromError(err)
	if !ok {
		return http.StatusServiceUnavailable, "cache not available"
	}
	switch status.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest, status.Message()
	case codes.AlreadyExists:
		return http.StatusConflict, status.Message()
	case codes.NotFound, codes.Unauthenticated:
		return http.StatusUnauthorized, status.Message()
	case codes.Unavailable:
		return http.StatusServiceUnavailable, status.Message()
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, status.Message()
	default:
		return http.StatusBadGateway, status.Message()
	}
}

func gatewayErrorToHTTP(err error) (int, string) {
	status, ok := grpcstatus.FromError(err)
	if !ok {
		return http.StatusServiceUnavailable, "gateway not available"
	}
	switch status.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest, status.Message()
	case codes.Unavailable:
		return http.StatusServiceUnavailable, status.Message()
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, status.Message()
	default:
		return http.StatusBadGateway, status.Message()
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, &errorRes{Error: message})
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		xlog.GLog.Warnf("write http response failed: %v", err)
	}
}
