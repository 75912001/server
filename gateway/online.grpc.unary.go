package main

import (
	"context"
	"strings"
	"time"

	"server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	xutil "github.com/75912001/xlib/util"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// handleAccountVerifyReq 处理客户端 AccountVerifyReq，并由 gateway 编排在线 session。
func handleAccountVerifyReq(
	remote xnetcommon.IRemote,
	header *xpacket.Header,
	body []byte,
) error {
	var verifyReq pb.AccountVerifyReq
	if err := proto.Unmarshal(body, &verifyReq); err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Unmarshal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "AccountVerifyReq unmarshal fail %v", xruntime.Location())
	}

	aid := verifyReq.GetAid()
	connectTicket := verifyReq.GetConnectTicket()
	if aid == 0 || connectTicket == "" {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.InvalidArgument.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.InvalidArgument, "AccountVerifyReq invalid aid or connectTicket %v", xruntime.Location())
	}

	ticketPayload, err := common.VerifyConnectTicket(connectTicket, common.ConnectTicketVerifyOptions{
		Secret:     GCfgCustomTicketSecret,
		GatewayKey: xetcd.GEtcd.GetKey(),
		AID:        aid,
		Now:        time.Now(),
	})
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Unauthenticated.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Unauthenticated, "connectTicket invalid aid:%v err:%v %v", aid, err, xruntime.Location())
	}

	if ticketPayload.Account == "" {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Unauthenticated.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Unauthenticated, "connectTicket payload invalid aid:%v %v", aid, xruntime.Location())
	}

	accountSession, err := xutil.RandomHex32()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Internal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "new accountSession failed aid:%v %v", aid, xruntime.Location())
	}

	oldSession, err := unaryCacheGetAccountSession(aid)
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), grpcErrorToResultCode(err), header.Key, nil)
		return errors.WithMessagef(err, "CacheGetAccountSession failed aid:%v %v", aid, xruntime.Location())
	}
	if oldSession != nil {
		if err = kickOldAccountSession(aid, oldSession); err != nil {
			_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), grpcErrorToResultCode(err), header.Key, nil)
			return errors.WithMessagef(err, "phase=kick_old aid=%v gatewayKey=%v accountSession=%s %v",
				aid, oldSession.GetGatewayKey(), oldSession.GetAccountSession(), xruntime.Location())
		}
	}

	gatewayKey := xetcd.GEtcd.GetKey()
	online, err := GOnlineMgr.ReserveByAvailableLoad()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Unavailable.Code(), header.Key, nil)
		return errors.WithMessagef(err, "select online for login aid:%v account:%v fail %v", aid, ticketPayload.Account, xruntime.Location())
	}

	heartbeatSession, err := xutil.RandomHex32()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Internal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "phase=new_heartbeat_session aid=%v gatewayKey=%s onlineKey=%s accountSession=%s %v",
			aid, gatewayKey, online.Key, accountSession, xruntime.Location())
	}

	u := GAccountMgr.Get(remote)
	if u == nil || !remote.IsConnect() {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Disconnect.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Disconnect, "remote not connect account:%v aid:%v %v", ticketPayload.Account, aid, xruntime.Location())
	}

	if err = unaryCacheBeginAccountSession(aid, "",
		&pb.CacheAccountSession{
			GatewayKey:       gatewayKey,
			AccountSession:   accountSession,
			LoginTimestampMs: time.Now().UnixMilli(),
			OnlineKey:        online.Key,
		},
	); err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), grpcErrorToResultCode(err), header.Key, nil)
		return errors.WithMessagef(err, "phase=begin_session aid=%v gatewayKey=%s onlineKey=%s accountSession=%s %v",
			aid, gatewayKey, online.Key, accountSession, xruntime.Location())
	}

	_, err = pb.NewOnlineServiceClient(online.GetClientConn()).OnlineBindAccount(context.Background(),
		&pb.OnlineBindAccountReq{
			Aid:            aid,
			Account:        ticketPayload.Account,
			GatewayKey:     gatewayKey,
			ClientIp:       remote.GetIP(),
			AccountSession: accountSession,
		},
	)
	if err != nil {
		cleanupGatewayBindSession(online, aid, accountSession, "online bind failed")
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), grpcErrorToResultCode(err), header.Key, nil)
		if status, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "phase=online_bind aid=%v gatewayKey=%s onlineKey=%s accountSession=%s code=%v message=%s %v",
				aid, gatewayKey, online.Key, accountSession, status.Code(), status.Message(), xruntime.Location())
		}
		return errors.WithMessagef(err, "phase=online_bind aid=%v gatewayKey=%s onlineKey=%s accountSession=%s %v",
			aid, gatewayKey, online.Key, accountSession, xruntime.Location())
	}

	if err = u.PostSyncVerified(aid, ticketPayload.Account, online, heartbeatSession, accountSession); err != nil {
		cleanupGatewayBindSession(online, aid, accountSession, "gateway bind failed after online bind")
		_ = sendClientRes(remote, uint32(pb.MsgID_AccountVerifyRes_CMD), xerror.Fail.Code(), header.Key, nil)
		return errors.WithMessagef(err, "account post verified account:%s aid:%d fail %v", ticketPayload.Account, aid, xruntime.Location())
	}

	xlog.GLog.Tracef("phase=verify_success aid=%d gatewayKey=%s onlineKey=%s accountSession=%s", aid, gatewayKey, online.Key, accountSession)
	return sendClientRes(remote,
		uint32(pb.MsgID_AccountVerifyRes_CMD),
		xerror.Success.Code(),
		header.Key,
		&pb.AccountVerifyRes{
			ServerTimestampMs: time.Now().UnixMilli(),
			HeartbeatSession:  heartbeatSession,
		},
	)
}

func kickOldAccountSession(aid uint64, oldSession *pb.CacheAccountSession) error {
	gatewayKey := strings.TrimSpace(oldSession.GetGatewayKey())
	accountSession := oldSession.GetAccountSession()
	if gatewayKey == "" || accountSession == "" {
		return grpcstatus.Error(codes.InvalidArgument, "old account session invalid")
	}
	if gatewayKey == xetcd.GEtcd.GetKey() {
		return kickLocalAccountSession(aid, accountSession)
	}

	peer := GGatewayPeerMgr.Get(gatewayKey)
	if peer == nil {
		return grpcstatus.Errorf(codes.Unavailable, "old gateway not found key:%s", gatewayKey)
	}
	client, err := peer.Client()
	if err != nil {
		return grpcstatus.Errorf(codes.Unavailable, "old gateway client unavailable key:%s err:%v", gatewayKey, err)
	}
	_, err = client.GatewayKickAccountSession(context.Background(), &pb.GatewayKickAccountSessionReq{
		Aid:            aid,
		Reason:         uint32(xnetcommon.DisconnectReasonServerShutdown),
		Msg:            "duplicate login",
		AccountSession: accountSession,
	})
	return err
}

func kickLocalAccountSession(aid uint64, accountSession string) error {
	account := GAccountMgr.GetByAID(aid)
	if account == nil {
		return grpcstatus.Errorf(codes.NotFound, "not found aid:%d", aid)
	}
	if account.accountSession != accountSession {
		return grpcstatus.Errorf(codes.Aborted, "account session changed aid:%d", aid)
	}
	account.remote.SetDisconnectReason(xnetcommon.DisconnectReasonServerShutdown)
	if _, err := GAccountMgr.Remove(account.remote); err != nil {
		return grpcstatus.Errorf(codes.FailedPrecondition, "kick cleanup failed aid:%d err:%v", aid, err)
	}
	return nil
}

func cleanupGatewayBindSession(online *Online, aid uint64, accountSession string, msg string) {
	gatewayKey := xetcd.GEtcd.GetKey()

	if err := unaryOnlineUnbindAccount(online, aid, gatewayKey, accountSession, xnetcommon.DisconnectReasonServerShutdown, msg); err != nil {
		xlog.GLog.Warnf("phase=cleanup_online aid=%d gatewayKey=%s onlineKey=%s accountSession=%s reason=%s err=%v",
			aid, gatewayKey, online.Key, accountSession, msg, err)
	}

	if err := unaryCacheEndAccountSession(aid, accountSession); err != nil {
		xlog.GLog.Warnf("phase=cleanup_cache aid=%d gatewayKey=%s accountSession=%s reason=%s err=%v",
			aid, gatewayKey, accountSession, msg, err)
	}
}

// grpcErrorToResultCode 映射 gRPC 错误码到 gateway 内部错误码。
func grpcErrorToResultCode(err error) uint32 {
	return common.GRPCStatusToResultID(err)
}

func unaryOnlineUnbindAccount(online *Online, aid uint64, gatewayKey string, accountSession string, reason xnetcommon.DisconnectReason, msg string) error {
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUnbindAccount(context.Background(),
		&pb.OnlineUnbindAccountReq{
			Aid:            aid,
			Reason:         uint32(reason),
			Msg:            msg,
			GatewayKey:     gatewayKey,
			AccountSession: accountSession,
		})
	return err
}
