package main

import (
	pb "server/proto/pb"
)

type gatewayGRPCServer struct {
	pb.UnimplementedGatewayServiceServer
}
