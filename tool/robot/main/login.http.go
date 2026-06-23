package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type loginAccountVerifyTokenReq struct {
	Account            string `json:"account"`
	AccountVerifyToken string `json:"accountVerifyToken"`
}

type loginSessionRes struct {
	Account                 string `json:"account"`
	Uid                     uint64 `json:"uid"`
	ConnectTicket           string `json:"connectTicket"`
	TicketExpireTimestampMs int64  `json:"ticketExpireTimestampMs"`
	GatewayKey              string `json:"gatewayKey"`
	GatewayAddr             string `json:"gatewayAddr"`
}

type loginErrorRes struct {
	Error string `json:"error"`
}

func loginCreateAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string) error {
	var out struct{}
	return postLoginJSON(ctx, GConfigYaml.Login.AccountVerifyTokenPath, &loginAccountVerifyTokenReq{
		Account:            account,
		AccountVerifyToken: accountVerifyToken,
	}, &out)
}

func loginUseAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string) (*loginSessionRes, error) {
	var out loginSessionRes
	if err := postLoginJSON(ctx, GConfigYaml.Login.SessionPath, &loginAccountVerifyTokenReq{
		Account:            account,
		AccountVerifyToken: accountVerifyToken,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func postLoginJSON(ctx context.Context, path string, in any, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, GConfigYaml.Login.Timeout)
	defer cancel()

	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, GConfigYaml.Login.Addr+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var errRes loginErrorRes
		_ = json.NewDecoder(resp.Body).Decode(&errRes)
		if errRes.Error == "" {
			errRes.Error = resp.Status
		}
		return fmt.Errorf("login http %s failed: %s", path, errRes.Error)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
