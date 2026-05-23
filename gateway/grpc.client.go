package main

import (
	"sync"

	"google.golang.org/grpc"
)

// grpcClient 实现 xgrpcutil.IClientConn，包装一条 gRPC 连接，供各服务 Mgr 使用
type grpcClient struct {
	id        string
	conn      *grpc.ClientConn
	available bool
	mu        sync.RWMutex
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
