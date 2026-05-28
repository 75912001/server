package main

import (
	"context"
	"fmt"
	"strconv"

	common "server/common"
	pb "server/proto/pb"

	xconfig "github.com/75912001/xlib/config"
	xetcd "github.com/75912001/xlib/etcd"
	xetcdconstants "github.com/75912001/xlib/etcd/constants"
	xgrpcproto "github.com/75912001/xlib/grpc/proto"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
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
		xlog.PrintfErr("UserVerifyReq unmarshal req: %v", err)
		return err
	}
	req := &pb.OnlineUserOnlineReq{
		Uid:       verifyReq.GetUid(),
		Token:     verifyReq.GetToken(),
		GatewayId: xetcd.GenKey(*xconfig.GConfigMgr.Base.ProjectName, xetcdconstants.WatchMsgTypeServer, *xconfig.GConfigMgr.Base.GroupID, *xconfig.GConfigMgr.Base.Name, *xconfig.GConfigMgr.Base.ServerID),
		ClientIp:  remote.GetIP(),
	}

	// 不在此处套 WithTimeout：拨号时已挂 TimeOutClientInterceptor，会按 proto methodOpt.timeout
	// （online.grpc.proto:21 → 60s）自动给 ctx 加 deadline；外层再 WithTimeout 取最早值会反而覆盖配置。
	// GXOnlineServiceService 是全局 *XOnlineServiceClient；OnlineUserOnline 内部
	// 调用 selector.Sel → xgrpcresolve.GetClientConnByHashRing 按 uid 选取连接，
	// 不使用 receiver 的 Client 字段，因此可直接用空实例。
	res, err := pb.GXOnlineServiceService.OnlineUserOnline(context.Background(), req)
	if err != nil {
		xlog.PrintfErr("OnlineUserOnline rpc error: %v", err)
		return err
	}
	xlog.PrintInfo(fmt.Sprintf("OnlineUserOnline uid=%d code=%d msg=%s",
		req.GetUid(), res.GetCode(), res.GetMsg()))

	// 校验通过：绑定 User 到本次哈希命中的 online 实例（GetByShardKey 与 selector.Sel 同一哈希环，结果一致），
	// 停止「未校验超时」定时器，启动心跳超时定时器。
	if res.GetCode() == 0 {
		u := GUserMgr.Get(remote)
		if u == nil || !remote.IsConnect() {
			xlog.PrintInfo(fmt.Sprintf("OnlineUserOnline uid=%d ignored, client disconnected", req.GetUid()))
			return nil
		}
		online, oerr := GOnlineMgr.GetByShardKey(fmt.Sprint(req.GetUid()))
		if oerr != nil {
			xlog.PrintfErr("OnlineUserOnline lookup online by uid=%d failed: %v", req.GetUid(), oerr)
			res.Code = common.ECGatewayOnlineNotFound.Code()
			res.Msg = common.ECGatewayOnlineNotFound.Error()
		} else if verr := u.PostSyncVerified(req.GetUid(), online); verr != nil {
			xlog.PrintfErr("OnlineUserOnline post verified uid=%d failed: %v", req.GetUid(), verr)
			res.Code = common.ECGatewayOnlineNotFound.Code()
			res.Msg = common.ECGatewayOnlineNotFound.Error()
		}
	}

	return remote.Send(&xpacket.Packet{
		Header: &xpacket.Header{
			MessageID: uint32(pb.MsgIDUser_UserVerifyRes_CMD),
			SessionID: header.SessionID,
			ResultID:  res.GetCode(),
			Key:       header.Key,
		},
		PBMessage: &pb.UserVerifyRes{
			ServerTime: res.GetServerTime(),
		},
	})
}

func unaryOnlineUserOffline(online *Online, uid uint64, reason xnetcommon.DisconnectReason, msg string) error {
	ctx := xgrpcproto.SetFromOutgoingContext(context.Background(), xgrpcproto.ShardKeyFieldNameDefault, strconv.FormatUint(uid, 10))
	_, err := pb.NewOnlineServiceClient(online.GetClientConn()).OnlineUserOffline(ctx, &pb.OnlineUserOfflineReq{
		Uid:    uid,
		Reason: uint32(reason),
		Msg:    msg,
	})
	return err
}
