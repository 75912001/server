# Cache 服务测试指南

## 适用范围

修改 cache 服务、cache gRPC handler、Redis 访问、cache proto 定义，或直接调用 cache RPC 的代码时，使用本文档。

## 快速检查

```bash
go test ./cache ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./cache
```

## 依赖检查

当修改 cache RPC 请求/响应结构、gRPC status 行为、Redis session 语义或生成的 proto 文件时，运行：

```bash
go test ./cache ./online ./gateway ./login ./tool/robot/main ./proto/pb
```

## 运行时依赖

手动验证 cache RPC 需要：

- `bin/cache.yaml` 中的 etcd 地址
- `bin/cache.yaml` 中的 Redis Cluster 地址
- `bin/cache.yaml` 中的 cache gRPC 监听地址

## 手动验证

典型检查项：

- account token 设置和消费流程
- user record 读写
- user session set/get/replace/delete/expire
- 参数错误返回 `InvalidArgument`
- 记录不存在返回 `NotFound`
- expected session 过期或不匹配返回 `Aborted`
