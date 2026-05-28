package main

import (
	pb "server/proto/pb"
)

type onlineGRPCServer struct {
	pb.UnimplementedOnlineServiceServer
}
