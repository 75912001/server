package main

import pb "server/proto/pb"

type cacheGRPCServer struct {
	pb.UnimplementedCacheServiceServer
}
