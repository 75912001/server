# Gateway 测试说明

本文档说明 `gateway` 服务的本地测试、覆盖率、性能测试和手工联调流程。命令默认在 Git Bash 中执行。

## 1. 快速单元测试

在仓库根目录执行：

```bash
cd /d/src/github.com/server
GOCACHE="$PWD/.gocache" go test ./gateway
```

当前 `gateway` 单元测试不依赖真实 TCP、etcd、online、cache 或 Redis，主要覆盖：

- `GatewayKickUser` 的参数校验、用户不存在、用户会话变更保护。
- `grpcErrorToResultCode` 对 gRPC 标准错误码到客户端 `ResultID` 的映射。
- `grpcErrorToResultCode` 的轻量性能基准。

## 2. 依赖包编译检查

修改 `gateway` 与协议、公共错误码、online 交互逻辑时，建议同时检查关联包：

```bash
cd /d/src/github.com/server
GOCACHE="$PWD/.gocache" go test ./common ./proto/pb ./gateway ./online
```

如果只需要确认 gateway 可独立编译：

```bash
cd /d/src/github.com/server/gateway
GOCACHE="$PWD/../.gocache" go test .
```

## 3. 覆盖率

```bash
cd /d/src/github.com/server
mkdir -p .coverage
GOCACHE="$PWD/.gocache" go test -coverprofile=.coverage/gateway.out ./gateway
go tool cover -func=.coverage/gateway.out
```

生成 HTML 覆盖率报告：

```bash
go tool cover -html=.coverage/gateway.out -o .coverage/gateway.html
```

## 4. Race 检查

`gateway` 存在 actor、TCP 回调、stream 回调、timer 回调等并发路径。涉及用户生命周期、online stream、GatewayKickUser、UserMgr、CacheMgr 或 OnlineMgr 时，建议执行 race 检查：

```bash
cd /d/src/github.com/server
GOCACHE="$PWD/.gocache" go test -race ./gateway
```

## 5. Benchmark

```bash
cd /d/src/github.com/server
GOCACHE="$PWD/.gocache" go test -bench=. -benchmem ./gateway
```

当前 benchmark 只覆盖 gRPC 错误码转换。后续如果要压测 TCP 包解析、UserMgr 分片、stream actor 发送路径，应新增独立 benchmark，避免混入真实网络依赖。

## 6. 手工联调

完整链路需要 gateway、online、cache、etcd、Redis 都处于可用状态。

推荐顺序：

```bash
cd /d/src/github.com/server
docker ps
```

确认依赖容器和服务运行后，使用客户端模拟器连接 gateway：

```bash
cd /d/src/github.com/server/tool/robot/bin
./robot.exe
```

重点验证：

- 输入 `UserVerifyReq` 后，客户端收到 `UserVerifyRes`，并且 `ResultID == 0`。
- 登录成功后，客户端按配置自动发送 `UserHeartbeatReq`，服务端返回 `UserHeartbeatRes.next_heartbeat_session`。
- 输入业务命令时，gateway 通过当前用户绑定的 online 透传上行包。
- 输入 `UserOfflineReq` 或由新 gateway 调用旧 gateway `GatewayKickUser` 时，gateway 清理本地 user，并通知绑定的 online 下线，同时按 CAS 删除 cache session。
- gateway stream 建立后，online 能识别 `gateway_id` 并绑定下行 stream。

## 7. 常见失败定位

- `selector for method ... not exist`：检查 online 是否已注册到 etcd，且 gateway 是否收到 online 的 etcd add/update 消息。
- `shard key not found`：检查对应 unary 调用是否传入 selector 需要的 key，或是否应该指定用户已绑定的 online。
- `GatewayKickUser` 返回 `InvalidArgument`：检查 `uid` 和 `user_session` 是否为空。
- `GatewayKickUser` 返回 `Aborted`：表示请求中的 `user_session` 已不是当前连接的会话，通常是重复登录或旧请求晚到。
- 客户端无响应：检查 gateway 日志、online 日志、client.simulator 的 `bin/log`，确认包头长度、消息 ID、session 和 key 是否正确。

## 8. 后续待补测试

- `User.OnClientPacket`：未验证包、心跳包、离线包、业务包分流。
- `User.Cleanup`：断线后只通知一次 online，并正确清理 `userSession` 对应的 cache session。
- `OnlineMgr`：etcd add/update/remove 与 stream actor Stop 的生命周期。
- `OnlineStreamTunnelPre/Post`：gateway stream 注册包发送与 reset 行为。
- `sendClientRes`：remote 断开、nil message、ResultID 透传。
