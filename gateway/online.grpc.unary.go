package main

import (
	"context"
	"time"

	"server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// unaryOnlineUserOnline 处理控制面 Unary RPC：OnlineUserOnline
func unaryOnlineUserOnline(
	remote xnetcommon.IRemote,
	header *xpacket.Header,
	body []byte,
) error {
	var verifyReq pb.UserVerifyReq
	if err := proto.Unmarshal(body, &verifyReq); err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Unmarshal.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "unaryOnlineUserOnline unmarshal fail %v", xruntime.Location())
	}
	uid := verifyReq.GetUid()
	gatewayNonce := verifyReq.GetGatewayNonce()
	gatewaySession := verifyReq.GetGatewaySession()
	if uid == 0 || gatewayNonce == "" || gatewaySession == "" {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.InvalidArgument.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(xerror.InvalidArgument, "OnlineUserOnline invalid uid, gatewayNonce or gatewaySession %v", xruntime.Location())
	}
	if common.NewGatewaySession(uid, xetcd.GEtcd.GetKey(), gatewayNonce) != gatewaySession {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Unauthenticated.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(xerror.Unauthenticated, "OnlineUserOnline gatewaySession mismatch uid:%v %v", uid, xruntime.Location())
	}

	pending, ok := GLoginSessionMgr.Consume(uid, gatewaySession)
	if !ok || pending == nil || pending.account == "" || pending.gatewayNonce != gatewayNonce {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Unauthenticated.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(xerror.Unauthenticated, "OnlineUserOnline pending session not found uid:%v %v", uid, xruntime.Location())
	}
	userSession, err := common.NewUserSession()
	if err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Internal.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "new userSession failed uid:%v %v", uid, xruntime.Location())
	}
	req := &pb.OnlineUserOnlineReq{
		Uid:            uid,
		Account:        pending.account,
		GatewayKey:     xetcd.GEtcd.GetKey(),
		ClientIp:       remote.GetIP(),
		GatewaySession: gatewaySession,
		UserSession:    userSession,
	}

	online, err := GOnlineMgr.GetForLogin()
	if err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Unavailable.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "OnlineUserOnline select online for login uid:%v account:%v fail %v", req.GetUid(), req.GetAccount(), xruntime.Location())
	}

	_, err = pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUserOnline(context.Background(), req)
	if err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			grpcErrorToResultCode(err),
			header.Key,
			nil,
		)
		status, ok := grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "OnlineUserOnline rpc error: %v, status code: %v, message: %v %v", err, status.Code(), status.Message(), xruntime.Location())
		}
		return errors.WithMessagef(err, "OnlineUserOnline rpc error: %v, %v", err, xruntime.Location())
	}

	account := req.GetAccount()
	onlineGatewaySession := req.GetGatewaySession()
	onlineUserSession := req.GetUserSession()
	xlog.GLog.Tracef("OnlineUserOnline account:%s uid:%d", account, uid)

	// 校验通过：绑定 User 到 online 实例
	// 停止「未校验超时」定时器，启动心跳超时定时器。
	u := GUserMgr.Get(remote)
	if u == nil || !remote.IsConnect() {
		cleanupOnlineLoginGatewaySession(online, uid, req.GetGatewayKey(), onlineGatewaySession, onlineUserSession, "gateway remote not connected after online login")
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Disconnect.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(xerror.Disconnect, "OnlineUserOnline remote not connect account:%v uid:%v %v", account, uid, xruntime.Location())
	}

	if err = u.PostSyncVerified(uid, account, online, onlineGatewaySession, onlineUserSession); err != nil {
		cleanupOnlineLoginGatewaySession(online, uid, req.GetGatewayKey(), onlineGatewaySession, onlineUserSession, "gateway bind failed after online login")
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Fail.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "OnlineUserOnline post verified account:%s uid:%d fail %v", account, uid, xruntime.Location())
	}

	return sendClientRes(remote,
		uint32(pb.MsgIDUser_UserVerifyRes_CMD),
		header.SessionID,
		xerror.Success.Code(),
		header.Key,
		&pb.UserVerifyRes{
			ServerTime: time.Now().UnixMilli(),
		},
	)
}

func cleanupOnlineLoginGatewaySession(online *Online, uid uint64, gatewayKey string, gatewaySession string, userSession string, msg string) {
	if online == nil || uid == 0 || gatewayKey == "" || gatewaySession == "" || userSession == "" {
		return
	}
	if err := unaryOnlineUserOffline(online, uid, gatewayKey, gatewaySession, userSession, xnetcommon.DisconnectReasonServerShutdown, msg); err != nil {
		xlog.GLog.Warnf("cleanup online login session failed uid:%d online:%s err:%v", uid, online.Key, err)
	}
}

func unaryOnlineUserUpdateGatewaySession(online *Online, uid uint64, gatewayKey string, oldGatewaySession string, newGatewaySession string, userSession string) error {
	if online == nil {
		return errors.Errorf("online is nil")
	}
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUserUpdateGatewaySession(context.Background(),
		&pb.OnlineUserUpdateGatewaySessionReq{
			Uid:               uid,
			GatewayKey:        gatewayKey,
			OldGatewaySession: oldGatewaySession,
			NewGatewaySession: newGatewaySession,
			UserSession:       userSession,
		})
	return err
}

// grpcErrorToResultCode 映射 gRPC 错误码到 gateway 内部错误码。
func grpcErrorToResultCode(err error) uint32 {
	status, ok := grpcstatus.FromError(err)
	if !ok {
		return xerror.Fail.Code()
	}
	switch status.Code() {
	case codes.OK:
		return xerror.Success.Code()
	case codes.Canceled:
		return xerror.Cancelled.Code()
	case codes.Unknown:
		return xerror.Unknown.Code()
	case codes.InvalidArgument:
		return xerror.InvalidArgument.Code()
	case codes.DeadlineExceeded:
		return xerror.DeadlineExceeded.Code()
	case codes.NotFound:
		return xerror.NotFound.Code()
	case codes.AlreadyExists:
		return xerror.AlreadyExists.Code()
	case codes.PermissionDenied:
		return xerror.PermissionDenied.Code()
	case codes.ResourceExhausted:
		return xerror.ResourceExhausted.Code()
	case codes.FailedPrecondition:
		return xerror.FailedPrecondition.Code()
	case codes.Aborted:
		return xerror.Aborted.Code()
	case codes.OutOfRange:
		return xerror.OutOfRange.Code()
	case codes.Unimplemented:
		return xerror.Unimplemented.Code()
	case codes.Internal:
		return xerror.Internal.Code()
	case codes.Unavailable:
		return xerror.Unavailable.Code()
	case codes.DataLoss:
		return xerror.DataLoss.Code()
	case codes.Unauthenticated:
		return xerror.Unauthenticated.Code()
	default:
		return xerror.Fail.Code()
	}
}

func unaryOnlineUserOffline(online *Online, uid uint64, gatewayKey string, gatewaySession string, userSession string, reason xnetcommon.DisconnectReason, msg string) error {
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUserOffline(context.Background(),
		&pb.OnlineUserOfflineReq{
			Uid:            uid,
			Reason:         uint32(reason),
			Msg:            msg,
			GatewayKey:     gatewayKey,
			GatewaySession: gatewaySession,
			UserSession:    userSession,
		})
	return err
}
