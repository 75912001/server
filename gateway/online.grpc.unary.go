package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xetcd "github.com/75912001/xlib/etcd"
	xgrpcproto "github.com/75912001/xlib/grpc/proto"
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
	req := &pb.OnlineUserOnlineReq{
		Uid:        uid,
		Token:      verifyReq.GetToken(),
		GatewayKey: xetcd.GEtcd.GetKey(),
		ClientIp:   remote.GetIP(),
	}

	online, err := GOnlineMgr.GetByAvailableLoad()
	if err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Unavailable.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "OnlineUserOnline select online by available load uid:%v fail %v", req.GetUid(), xruntime.Location())
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

	xlog.GLog.Tracef(fmt.Sprintf("OnlineUserOnline uid:%d", uid))

	// 校验通过：绑定 User 到 online 实例
	// 停止「未校验超时」定时器，启动心跳超时定时器。
	u := GUserMgr.Get(remote)
	if u == nil || !remote.IsConnect() {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Disconnect.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "OnlineUserOnline remote not connect uid:%v %v", uid, xruntime.Location())
	}

	if err = u.PostSyncVerified(req.GetUid(), online); err != nil {
		_ = sendClientRes(
			remote,
			uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			header.SessionID,
			xerror.Fail.Code(),
			header.Key,
			nil,
		)
		return errors.WithMessagef(err, "OnlineUserOnline post verified uid:%d fail %v", req.GetUid(), xruntime.Location())
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

// todo menglc 目前 grpc 错误码与网关内部错误码的映射关系比较粗糙，可以根据实际业务需求细化
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

func unaryOnlineUserOffline(online *Online, uid uint64, reason xnetcommon.DisconnectReason, msg string) error {
	ctx := xgrpcproto.SetFromOutgoingContext(context.Background(), xgrpcproto.ShardKeyFieldNameDefault, strconv.FormatUint(uid, 10))
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUserOffline(ctx,
		&pb.OnlineUserOfflineReq{
			Uid:    uid,
			Reason: uint32(reason),
			Msg:    msg,
		})
	return err
}
