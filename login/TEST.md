# Login 服务测试指南

## 适用范围

修改以下内容时使用本文档：

- login HTTP handler、请求解析和错误码映射。
- accountVerifyToken 写入、验证和消费流程。
- etcd 发现 cache/gateway 的逻辑。
- gateway 选择规则。
- `connectTicket` payload、签名、过期时间或共享密钥。
- login 配置、部署配置或 robot 登录行为。

## 快速检查

只修改 login 文档或不影响协议的 login 内部实现时，优先运行：

```bash
go test ./login ./common ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./login
```

## 依赖检查

修改 accountVerifyToken、cache RPC、gateway 验票、online 登录链路或 robot 自动登录时，运行：

```bash
go test ./login ./cache ./gateway ./online ./tool/robot/main ./common ./proto/pb
```

修改 proto 后先生成代码：

```bash
python gen.py
go test ./login ./cache ./gateway ./online ./tool/robot/main ./common ./proto/pb
```

## 静态检查

检查 login 当前接口和客户端字段是否保持 `connectTicket` 语义：

```bash
rg -n "connectTicket|connect_ticket|CacheSetAccountVerifyToken|CacheUseAccountVerifyToken" login common gateway proto tool
rg -n "GatewayPrepareLogin|gatewaySession|gatewayNonce" login proto gateway
```

期望：

- login 中 `/api/login/session` 返回 `connectTicket`，不返回 `gatewaySession/gatewayNonce`。
- `UserVerifyReq` 使用 `uid + connect_ticket`。
- login 不调用 `GatewayPrepareLogin`。

## 运行时依赖

手动验证 login 需要启动：

- etcd：login 用于发现 cache 和 gateway。
- Redis：cache 保存 accountVerifyToken、账号 uid 映射、用户档案和在线态。
- cache：提供 `CacheSetAccountVerifyToken` 和 `CacheUseAccountVerifyToken`。
- gateway：注册到 etcd，并暴露客户端 TCP 地址。
- login：读取 `bin/login.yaml` 或部署目录中的 login yaml。

`bin/login.yaml` 当前只必须显式配置 `custom.httpAddr`；`accountVerifyTokenPath/sessionPath/accountVerifyTokenExpireSecond/ticketExpireSecond/ticketSecret/readHeaderTimeout/shutdownTimeout/cacheRPCTimeout/maxBodyBytes` 都有代码默认值。

## HTTP 手动验证

写入 accountVerifyToken：

```bash
curl -i -X POST "http://127.0.0.1:30401/api/login/accountVerifyToken" \
  -H "Content-Type: application/json" \
  -d '{"account":"robot.10001","accountVerifyToken":"account-verify-token-value"}'
```

期望：

- HTTP `200`。
- 响应包含 `account/accountVerifyToken/expireSecond`。
- 不返回 `uid/connectTicket/gatewayKey/gatewayAddr`。
- cache 中只写 accountVerifyToken；此步骤不创建在线态。

使用 accountVerifyToken 换取连接票据：

```bash
curl -i -X POST "http://127.0.0.1:30401/api/login/session" \
  -H "Content-Type: application/json" \
  -d '{"account":"robot.10001","accountVerifyToken":"account-verify-token-value"}'
```

期望：

- HTTP `200`。
- 响应包含 `account/uid/connectTicket/ticketExpireAt/gatewayKey/gatewayAddr`。
- `uid` 由 cache 返回，客户端没有提交 uid。
- `connectTicket` 只可用于响应中的目标 gateway。
- 同一个 `account/accountVerifyToken` 再次调用 `/api/login/session` 应失败。

## 登录链路验证

使用 `/api/login/session` 返回的数据连接 gateway：

1. 客户端连接 `gatewayAddr`。
2. 客户端发送 `UserVerifyReq`，字段为 `uid` 和 `connect_ticket`。
3. gateway 使用本机 `gatewayKey`、配置中的 `ticketSecret` 和客户端 uid 校验 `connectTicket`。
4. 验签成功后 gateway 选择 online，执行在线登录和顶号流程。
5. gateway 返回 `UserVerifyRes`，其中包含后续心跳使用的 `heartbeatSession`。

期望：

- gateway 不信任客户端 account。
- gateway 不接受缺少 `connectTicket` 的登录请求。
- gateway 连接到错误的 gatewayKey 时，票据校验失败。
- `connectTicket` 过期后不能继续登录。
- `heartbeatSession` 由 gateway 生成，不在 login 响应中出现。

## 回归场景

- `/api/login/accountVerifyToken` 请求包含未知字段，返回 `400`。
- `/api/login/accountVerifyToken` 的 `account` trim 后为空，返回 `400`。
- `/api/login/accountVerifyToken` 重复写入未消费 accountVerifyToken，返回冲突。
- `/api/login/session` 使用错误 accountVerifyToken，返回失败。
- `/api/login/session` 成功后重复使用同一 accountVerifyToken，返回失败。
- 没有可用 cache 时，accountVerifyToken/session 接口返回 cache 不可用或超时。
- 没有可用 gateway 时，`/api/login/session` 返回 `503`。
- login 和 gateway 的 `ticketSecret` 不一致时，`/api/login/session` 成功但 gateway 登录失败。
- gateway `availableLoad` 变化后，login 选择新的最大可用负载 gateway。
- 双 gateway 同 `availableLoad` 时，批量 session 分配不应全部集中到同一个实例；选中实例的本地负载会先扣减，后续 etcd update 再覆盖本地估算值。

## Docker 验证

本地容器操作见 `deploy/README.md` 和 `deploy/login/README.md`。验证 login 容器时重点检查：

- `deploy/login/login.1.yaml`、`deploy/login/login.2.yaml` 只覆盖必要配置。
- 容器内 `/app/config/login.yaml` 中 `custom.httpAddr` 与端口映射一致。
- `server.login.1` 映射 `30401`，`server.login.2` 映射 `30402`。
- 容器日志中能看到 HTTP 监听地址和 etcd 发现 cache/gateway 的日志。
