package main

import (
	"context"
	"errors"
	"net/http"

	xlog "github.com/75912001/xlib/log"
	xutil "github.com/75912001/xlib/util"

	pb "server/proto/pb"
)

var errLoginCredentialConfigInvalid = errors.New("login credential config invalid")

// emailSessionReq 是客户端使用 email/password 登录时的请求体。
type emailSessionReq struct {
	Email    string `json:"email"`    // 邮箱账号, trim 后统一转小写
	Password string `json:"password"` // 明文密码, 精确匹配配置文件
}

// emailPasswordAccount 是 login 配置文件中的 email/password 登录账号。
type emailPasswordAccount struct {
	Email    string `yaml:"email"`    // 邮箱账号, trim 后统一转小写
	Password string `yaml:"password"` // 明文密码
}

// emailPasswordConfig 只解析 login 运行配置里的 custom.emailPasswordAccounts。
type emailPasswordConfig struct {
	Custom struct {
		EmailPasswordAccounts []emailPasswordAccount `yaml:"emailPasswordAccounts"`
	} `yaml:"custom"`
}

// handleLoginEmailSession 供客户端使用 email/password 换取 aid、connectTicket 和目标 gateway。
func handleLoginEmailSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req, ok := decodeEmailSessionReq(w, r)
	if !ok {
		return
	}

	accounts, err := loadEmailPasswordAccounts()
	if err != nil {
		if xlog.GLog != nil {
			xlog.GLog.Warnf("load email password accounts failed: %v", err)
		}
		writeError(w, http.StatusInternalServerError, errLoginCredentialConfigInvalid.Error())
		return
	}
	password, ok := accounts[req.Email]
	if !ok || password != req.Password {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	accountVerifyToken, err := xutil.RandomHex32()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "new account verify token failed")
		return
	}

	cacheCtx, cacheCancel := context.WithTimeout(r.Context(), GCfgCustomCacheRPCTimeout)
	defer cacheCancel()

	_, err = pb.GXCacheServiceService.CacheSetAccountVerifyToken(cacheCtx, &pb.CacheSetAccountVerifyTokenReq{
		Account:            req.Email,
		AccountVerifyToken: accountVerifyToken,
		ExpireSecond:       GCfgCustomAccountVerifyTokenExpireSecond,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}

	cacheRes, err := pb.GXCacheServiceService.CacheUseAccountVerifyToken(cacheCtx, &pb.CacheUseAccountVerifyTokenReq{
		Account:            req.Email,
		AccountVerifyToken: accountVerifyToken,
	})
	if err != nil {
		statusCode, message := cacheErrorToHTTP(err)
		writeError(w, statusCode, message)
		return
	}
	writeLoginSession(w, req.Email, cacheRes.GetAid())
}
