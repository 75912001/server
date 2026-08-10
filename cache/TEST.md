# Cache 服务测试指南

本文档中的命令使用 Git Bash 执行。每个命令块都会先进入当前仓库的 `cache` 目录，可以在 GoLand/Markdown 中直接点击运行。

## 测试目标

cache 测试重点不是单纯追求覆盖率，而是稳定覆盖服务契约：

- gRPC status 返回语义。
- Redis key 格式。
- accountVerifyToken 设置、消费和删除。
- aid 按 `base.groupID` 分段生成。
- `AccountRecord` protobuf 读写, 包括角色随身携带宠物和账号宠物仓库字段透传。
- 角色邮箱文本规范化、hash 字段解码、过期清理和损坏数据拒绝。
- cache accountSession 的批量读写、CAS 替换、续期、删除和迟到请求保护。

## 快速单元测试

不依赖 Redis、etcd、网络，适合每次修改后先跑：

```bash
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test ../common . ../proto/pb
```

覆盖内容：

- `common.GroupAIDStart` 公式。
- 系统邮件主题和正文长度、控制字符、换行规范化。
- cache Redis key 生成。
- 角色邮箱 `usedUUID/mail:{uuid}` 字段生成、protobuf 解码、过期过滤和损坏数据拒绝。
- cache accountSession enum 到 Redis hash field 的映射。
- cache accountSession identity/record 转换。
- cache accountSession response 组装。
- handler 参数错误对应的 gRPC status。

## 依赖编译检查

当修改 cache RPC、Redis 语义、公共 common 逻辑或 proto 生成代码时运行：

```bash
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test ../common . ../online ../gateway ../login ../proto/pb
```

```bash
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go build -buildvcs=false .
```

## Race 与覆盖率

Race 需要当前 Windows 环境可用 cgo 和 C 编译器，例如 `gcc`。如果缺少编译器，该命令会失败，属于环境限制。

```bash
cd ../cache 2>/dev/null || cd cache
PATH="/c/msys64/ucrt64/bin:$PATH" CGO_ENABLED=1 GOCACHE="$PWD/../.gocache" go test -race ../common .
```

```bash
cd ../cache 2>/dev/null || cd cache
mkdir -p ../.coverage
GOCACHE="$PWD/../.gocache" go test -coverprofile=../.coverage/cache.out ../common .
GOCACHE="$PWD/../.gocache" go tool cover -func=../.coverage/cache.out
```

覆盖率目标：

- `common`：90% 以上。
- cache helper/handler：优先覆盖参数校验和 status 语义。
- Redis 逻辑：以场景覆盖为主，不以普通覆盖率数字作为唯一目标。

## Redis 集成测试

集成测试需要真实 Redis Cluster，默认不运行，使用 `integration` build tag：

```bash
export CACHE_TEST_REDIS_ADDRS="192.168.71.123:7000,192.168.71.123:7001,192.168.71.123:7002,192.168.71.123:7003,192.168.71.123:7004,192.168.71.123:7005"
export CACHE_TEST_REDIS_PASSWORD="111111"
export CACHE_TEST_GROUP_ID="9"
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test -tags=integration .
```

注意事项：

- 只在测试 Redis Cluster 上运行，不要指向生产 Redis。
- 测试会写入并清理 `account:*`、`account:*` 和 `account:aid:sequence:{groupID}` 相关 key。
- 默认测试 groupID 为 `9`，也可以通过 `CACHE_TEST_GROUP_ID` 指定。

覆盖内容：

- accountVerifyToken 首次设置、重复设置、错误 accountVerifyToken、消费后删除。
- 新账号创建、重复读取、AccountRecord 缺失或关键字段不一致时报错。
- 同账号并发创建只生成一个 aid。
- cache accountSession HSET/HMGET。
- cache accountSession CAS replace、expire、delete。
- 旧 accountSession 的迟到 expire/delete 不影响新 cache accountSession。

## Benchmark

默认 benchmark 覆盖纯 helper：

```bash
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test -bench=. -benchmem .
```

Redis benchmark 需要真实 Redis Cluster：

```bash
export CACHE_TEST_REDIS_ADDRS="192.168.71.123:7000,192.168.71.123:7001,192.168.71.123:7002,192.168.71.123:7003,192.168.71.123:7004,192.168.71.123:7005"
export CACHE_TEST_REDIS_PASSWORD="111111"
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test -tags=integration -bench=. -benchmem .
```

关注指标：

- QPS。
- 平均延迟。
- P95/P99。
- Redis CPU/内存。
- cache 进程 CPU/内存。
- 并发账号创建时 aid 是否重复。

## 一键检查

需要一次性获得常规测试结果时运行：

```bash
cd ../cache 2>/dev/null || cd cache
GOCACHE="$PWD/../.gocache" go test ../common . ../proto/pb
GOCACHE="$PWD/../.gocache" go test ../common . ../online ../gateway ../login ../proto/pb
GOCACHE="$PWD/../.gocache" go build -buildvcs=false .
GOCACHE="$PWD/../.gocache" go test -bench=. -benchmem .
mkdir -p ../.coverage
GOCACHE="$PWD/../.gocache" go test -coverprofile=../.coverage/cache.out ../common .
GOCACHE="$PWD/../.gocache" go tool cover -func=../.coverage/cache.out
GOCACHE="$PWD/../.gocache" go test -tags=integration -run TestIntegration -v .
rm -f cache.exe ../.coverage/cache.out
rmdir ../.coverage 2>/dev/null || true
```

## 手动 gRPC 验证

手动验证 cache RPC 需要：

- `../bin/cache.yaml` 中的 etcd 地址。
- `../bin/cache.yaml` 中的 Redis Cluster 地址。
- `../bin/cache.yaml` 中的 cache gRPC 监听地址。

典型检查项：

- accountVerifyToken 设置和消费流程。
- account record 读写; cache 不校验宠物仓库和角色随身携带宠物之间的业务不变量。
- 角色邮箱 get/add/read/delete; 验证角色归属、365 天过期、1000 封有效邮件上限和只清理过期邮件。
- cache accountSession set/get/replace/delete/expire。
- 参数错误返回 `InvalidArgument`。
- 记录不存在返回 `NotFound`。
- expected accountSession 过期或不匹配返回 `Aborted`。
