package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xgrpcproto "github.com/75912001/xlib/grpc/proto"
	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func cacheVerifyToken(uid uint64) string {
	return fmt.Sprintf("robot.%d.%d", uid, time.Now().UnixNano())
}

func cacheSetVerifyUserToken(uid uint64) (string, error) {
	if uid == 0 {
		return "", xerror.InvalidArgument
	}
	token := cacheVerifyToken(uid)
	ctx := xgrpcproto.SetFromOutgoingContext(context.Background(), xgrpcproto.ShardKeyFieldNameDefault, strconv.FormatUint(uid, 10))
	_, err := pb.GXCacheServiceService.CacheSetVerifyUserToken(ctx, &pb.CacheSetVerifyUserTokenReq{
		Uid:          uid,
		Token:        token,
		ExpireSecond: GConfigYaml.CacheTokenExpire,
	})
	if err != nil {
		s, ok := grpcstatus.FromError(err)
		if ok {
			return "", errors.WithMessagef(err, "CacheSetVerifyUserToken uid:%d token:%s code:%v message:%s", uid, token, s.Code(), s.Message())
		}
		return "", errors.WithMessagef(err, "CacheSetVerifyUserToken uid:%d token:%s", uid, token)
	}
	return token, nil
}
