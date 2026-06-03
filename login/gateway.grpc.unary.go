package main

import (
	"context"

	pb "server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

func prepareGatewayLogin(ctx context.Context, gateway *Gateway, uid uint64, account string, gatewayNonce string, gatewaySession string) error {
	if gateway == nil || gateway.GrpcAddr == "" || gateway.XGatewayService == nil || !gateway.Available() {
		return errors.Errorf("gateway grpc service unavailable %v", xruntime.Location())
	}
	_, err := pb.NewGatewayServiceClient(gateway.GetClientConn()).GatewayPrepareLogin(ctx, &pb.GatewayPrepareLoginReq{
		Uid:            uid,
		Account:        account,
		GatewayNonce:   gatewayNonce,
		GatewaySession: gatewaySession,
		ExpireSecond:   GCfgCustomSessionExpireSecond,
	})
	return err
}
