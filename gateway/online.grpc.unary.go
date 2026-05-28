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
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// unaryOnlineUserOnline 处理控制面 Unary RPC：OnlineUserOnline
// selector.Sel 内部按 uid 哈希自动选取目标 online 实例，无需调用方预先选连接
func unaryOnlineUserOnline(
	remote xnetcommon.IRemote,
	header *xpacket.Header,
	body []byte,
) error {
	var verifyReq pb.UserVerifyReq
	if err := proto.Unmarshal(body, &verifyReq); err != nil {
		return errors.WithMessagef(err, "unaryOnlineUserOnline unmarshal fail %v", xruntime.Location())
	}
	uid := verifyReq.GetUid()
	req := &pb.OnlineUserOnlineReq{
		Uid:        uid,
		Token:      verifyReq.GetToken(),
		GatewayKey: xetcd.GEtcd.GetKey(),
		ClientIp:   remote.GetIP(),
	}

	_, err := pb.GXOnlineServiceService.OnlineUserOnline(context.Background(), req)
	if err != nil {
		status, ok := grpcstatus.FromError(err)
		if ok {
			return errors.WithMessagef(err, "OnlineUserOnline rpc error: %v, status code: %v, message: %v %v", err, status.Code(), status.Message(), xruntime.Location())
		}
		return errors.WithMessagef(err, "OnlineUserOnline rpc error: %v, %v", err, xruntime.Location())
	}

	xlog.GLog.Tracef(fmt.Sprintf("OnlineUserOnline uid:%d", uid))

	// 校验通过：绑定 User 到本次哈希命中的 online 实例
	// 停止「未校验超时」定时器，启动心跳超时定时器。
	u := GUserMgr.Get(remote)
	if u == nil || !remote.IsConnect() {
		return errors.WithMessagef(err, "OnlineUserOnline remote not connect uid:%v %v", uid, xruntime.Location())
	}
	// 选一个 online 实例，进行后续绑定和心跳管理. 理论上应该能找到与 pb.GXOnlineServiceService.OnlineUserOnline 相同的 online实例，因为它们都基于相同的 selector.Sel 和 uid 哈希算法，
	// todo menglc 但如果找不到/找到的不一致，说明在线服务实例发生了变更（重启或扩容），需要让用户重新上线以绑定新的实例。
	online, err := GOnlineMgr.GetByShardKey(fmt.Sprint(req.GetUid()))
	if err != nil {
		return errors.WithMessagef(err, "OnlineUserOnline lookup online by uid:%v fail %v", req.GetUid(), xruntime.Location())
	}
	if err = u.PostSyncVerified(req.GetUid(), online); err != nil {
		return errors.WithMessagef(err, "OnlineUserOnline post verified uid:%d fail %v", req.GetUid(), xruntime.Location())
	}

	return remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			SessionID: header.SessionID,
			ResultID:  xerror.Success.Code(),
			Key:       header.Key,
		},
		PBMessage: &pb.UserVerifyRes{
			ServerTime: time.Now().UnixMilli(),
		},
	})
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
