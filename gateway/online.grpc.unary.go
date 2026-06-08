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

// handleUserVerifyReq 处理客户端 UserVerifyReq，并由 gateway 编排在线 session。
func handleUserVerifyReq(
	remote xnetcommon.IRemote,
	header *xpacket.Header,
	body []byte,
) error {
	var verifyReq pb.UserVerifyReq
	if err := proto.Unmarshal(body, &verifyReq); err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Unmarshal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "UserVerifyReq unmarshal fail %v", xruntime.Location())
	}

	uid := verifyReq.GetUid()
	connectTicket := verifyReq.GetConnectTicket()
	if uid == 0 || connectTicket == "" {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.InvalidArgument.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.InvalidArgument, "UserVerifyReq invalid uid or connectTicket %v", xruntime.Location())
	}

	ticketPayload, err := common.VerifyConnectTicket(connectTicket, common.ConnectTicketVerifyOptions{
		Secret:     GCfgCustomTicketSecret,
		GatewayKey: xetcd.GEtcd.GetKey(),
		UID:        uid,
		Now:        time.Now(),
	})
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Unauthenticated.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Unauthenticated, "connectTicket invalid uid:%v err:%v %v", uid, err, xruntime.Location())
	}

	if ticketPayload.Account == "" {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Unauthenticated.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Unauthenticated, "connectTicket payload invalid uid:%v %v", uid, xruntime.Location())
	}

	userSession, err := xutil.RandomHex32()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Internal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "new userSession failed uid:%v %v", uid, xruntime.Location())
	}

	oldSession, err := unaryCacheGetUserSession(uid)
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, grpcErrorToResultCode(err), header.Key, nil)
		return errors.WithMessagef(err, "CacheGetUserSession failed uid:%v %v", uid, xruntime.Location())
	}
	if oldSession != nil {
		if err = kickOldUserSession(uid, oldSession); err != nil {
			_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, grpcErrorToResultCode(err), header.Key, nil)
			return errors.WithMessagef(err, "phase=kick_old uid=%v gatewayKey=%v userSession=%s %v",
				uid, oldSession.GetGatewayKey(), oldSession.GetUserSession(), xruntime.Location())
		}
	}

	gatewayKey := xetcd.GEtcd.GetKey()
	online, err := GOnlineMgr.GetByAvailableLoad()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Unavailable.Code(), header.Key, nil)
		return errors.WithMessagef(err, "select online for login uid:%v account:%v fail %v", uid, ticketPayload.Account, xruntime.Location())
	}

	heartbeatSession, err := xutil.RandomHex32()
	if err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Internal.Code(), header.Key, nil)
		return errors.WithMessagef(err, "phase=new_heartbeat_session uid=%v gatewayKey=%s onlineKey=%s userSession=%s %v",
			uid, gatewayKey, online.Key, userSession, xruntime.Location())
	}

	u := GUserMgr.Get(remote)
	if u == nil || !remote.IsConnect() {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Disconnect.Code(), header.Key, nil)
		return errors.WithMessagef(xerror.Disconnect, "remote not connect account:%v uid:%v %v", ticketPayload.Account, uid, xruntime.Location())
	}

	if err = unaryCacheBeginUserSession(uid, "",
		&pb.CacheUserSession{
			GatewayKey:  gatewayKey,
			UserSession: userSession,
			LoginTimeMs: time.Now().UnixMilli(),
			OnlineKey:   online.Key,
		},
	); err != nil {
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, grpcErrorToResultCode(err), header.Key, nil)
		return errors.WithMessagef(err, "phase=begin_session uid=%v gatewayKey=%s onlineKey=%s userSession=%s %v",
			uid, gatewayKey, online.Key, userSession, xruntime.Location())
	}

	_, err = pb.NewOnlineServiceClient(online.GetClientConn()).OnlineBindUser(context.Background(),
		&pb.OnlineBindUserReq{
			Uid:         uid,
			Account:     ticketPayload.Account,
			GatewayKey:  gatewayKey,
			ClientIp:    remote.GetIP(),
			UserSession: userSession,
		},
	)
	if err != nil {
		cleanupGatewayBindSession(online, uid, userSession, "online bind failed")
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, grpcErrorToResultCode(err), header.Key, nil)
		if status, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "phase=online_bind uid=%v gatewayKey=%s onlineKey=%s userSession=%s code=%v message=%s %v",
				uid, gatewayKey, online.Key, userSession, status.Code(), status.Message(), xruntime.Location())
		}
		return errors.WithMessagef(err, "phase=online_bind uid=%v gatewayKey=%s onlineKey=%s userSession=%s %v",
			uid, gatewayKey, online.Key, userSession, xruntime.Location())
	}

	if err = u.PostSyncVerified(uid, ticketPayload.Account, online, heartbeatSession, userSession); err != nil {
		cleanupGatewayBindSession(online, uid, userSession, "gateway bind failed after online bind")
		_ = sendClientRes(remote, uint32(pb.MsgIDUser_UserVerifyRes_CMD), header.SessionID, xerror.Fail.Code(), header.Key, nil)
		return errors.WithMessagef(err, "user post verified account:%s uid:%d fail %v", ticketPayload.Account, uid, xruntime.Location())
	}

	xlog.GLog.Tracef("phase=verify_success uid=%d gatewayKey=%s onlineKey=%s userSession=%s", uid, gatewayKey, online.Key, userSession)
	return sendClientRes(remote,
		uint32(pb.MsgIDUser_UserVerifyRes_CMD),
		header.SessionID,
		xerror.Success.Code(),
		header.Key,
		&pb.UserVerifyRes{
			ServerTime:       time.Now().UnixMilli(),
			HeartbeatSession: heartbeatSession,
		},
	)
}

func kickOldUserSession(uid uint64, oldSession *pb.CacheUserSession) error {
	gatewayKey := strings.TrimSpace(oldSession.GetGatewayKey())
	userSession := oldSession.GetUserSession()
	if gatewayKey == "" || userSession == "" {
		return grpcstatus.Error(codes.InvalidArgument, "old user session invalid")
	}
	if gatewayKey == xetcd.GEtcd.GetKey() {
		return kickLocalUserSession(uid, userSession)
	}

	peer := GGatewayPeerMgr.Get(gatewayKey)
	if peer == nil {
		return grpcstatus.Errorf(codes.Unavailable, "old gateway not found key:%s", gatewayKey)
	}
	client, err := peer.Client()
	if err != nil {
		return grpcstatus.Errorf(codes.Unavailable, "old gateway client unavailable key:%s err:%v", gatewayKey, err)
	}
	_, err = client.GatewayKickUser(context.Background(), &pb.GatewayKickUserReq{
		Uid:         uid,
		Reason:      uint32(xnetcommon.DisconnectReasonServerShutdown),
		Msg:         "duplicate login",
		UserSession: userSession,
	})
	return err
}

func kickLocalUserSession(uid uint64, userSession string) error {
	user := GUserMgr.GetByUID(uid)
	if user == nil {
		return grpcstatus.Errorf(codes.NotFound, "not found uid:%d", uid)
	}
	if user.userSession != userSession {
		return grpcstatus.Errorf(codes.Aborted, "user session changed uid:%d", uid)
	}
	user.remote.SetDisconnectReason(xnetcommon.DisconnectReasonServerShutdown)
	if _, err := GUserMgr.Remove(user.remote); err != nil {
		return grpcstatus.Errorf(codes.FailedPrecondition, "kick cleanup failed uid:%d err:%v", uid, err)
	}
	return nil
}

func cleanupGatewayBindSession(online *Online, uid uint64, userSession string, msg string) {
	gatewayKey := xetcd.GEtcd.GetKey()

	if err := unaryOnlineUnbindUser(online, uid, gatewayKey, userSession, xnetcommon.DisconnectReasonServerShutdown, msg); err != nil {
		xlog.GLog.Warnf("phase=cleanup_online uid=%d gatewayKey=%s onlineKey=%s userSession=%s reason=%s err=%v",
			uid, gatewayKey, online.Key, userSession, msg, err)
	}

	if err := unaryCacheEndUserSession(uid, userSession); err != nil {
		xlog.GLog.Warnf("phase=cleanup_cache uid=%d gatewayKey=%s userSession=%s reason=%s err=%v",
			uid, gatewayKey, userSession, msg, err)
	}
}

// grpcErrorToResultCode 映射 gRPC 错误码到 gateway 内部错误码。
func grpcErrorToResultCode(err error) uint32 {
	return common.GRPCStatusToResultID(err)
}

func unaryOnlineUnbindUser(online *Online, uid uint64, gatewayKey string, userSession string, reason xnetcommon.DisconnectReason, msg string) error {
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUnbindUser(context.Background(),
		&pb.OnlineUnbindUserReq{
			Uid:         uid,
			Reason:      uint32(reason),
			Msg:         msg,
			GatewayKey:  gatewayKey,
			UserSession: userSession,
		})
	return err
}
