package main

import (
	pb "server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type Cache struct {
	*pb.XCacheService
	Key string // etcd key

	GroupID     uint32
	ServerName  string
	ServerID    uint32
	PackageName string
	ServiceName string
}

func newCache(key, addr string) (*Cache, error) {
	xService, err := pb.NewXCacheService(addr)
	if err != nil {
		return nil, errors.WithMessage(err, xruntime.Location())
	}
	return &Cache{Key: key, XCacheService: xService}, nil
}

func (c *Cache) Client() pb.CacheServiceClient {
	return pb.NewCacheServiceClient(c.GetClientConn())
}

func (c *Cache) GetID() string { return c.Key }

func (c *Cache) GetClientConn() *grpc.ClientConn {
	return c.XCacheService.GetClientConn()
}
