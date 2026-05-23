package main

import (
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcClient 实现 xgrpcutil.IClientConn，包装一条 gRPC 连接，供各服务 Mgr 使用
type grpcClient struct {
	id        string           // 唯一标识，格式 "{groupID}.{serverName}.{serverID}"
	conn      *grpc.ClientConn // gRPC 底层连接，创建后只读，无需加锁
	available bool             // 服务是否可用；Disabled() 置 false 后不再被 resolve 选中
	mu        sync.RWMutex     // 保护 available 的并发读写
}

func newGrpcClient(id string, addr string) (*grpcClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &grpcClient{id: id, conn: conn, available: true}, nil
}

func (c *grpcClient) GetClientConn() *grpc.ClientConn {
	return c.conn
}

func (c *grpcClient) GetID() string {
	return c.id
}

func (c *grpcClient) Available() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.available
}

func (c *grpcClient) Disabled() {
	c.mu.Lock()
	c.available = false
	c.mu.Unlock()
}

func (c *grpcClient) Stop() error {
	c.Disabled()
	return c.conn.Close()
}
