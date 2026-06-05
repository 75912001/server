# Login 服务

Login 服务负责对外提供 HTTP 登录入口，完成一次性 token 的签发和消费，并为客户端分配可用 gateway。部署、端口、容器启动和验证命令见 `deploy/login/README.md`。

## 能力边界

- 提供 `POST /api/login/token`，接收外部程序提交的 `account` 和 `token`，通过 cache 写入一次性登录 token。
- 提供 `POST /api/login/session`，接收客户端提交的 `account` 和 `token`，通过 cache 验证并消费 token，获得可信 uid。
- 从 etcd 发现可用 gateway，按 `availableLoad` 最大优先选择；负载相同按 gateway key 字典序选择。
- 为客户端生成短期 `connectTicket`，票据内包含目标 gateway、uid、account、nonce 和过期时间。
- 返回客户端连接 gateway 所需的 `uid`、`gatewayKey`、`gatewayAddr`、`connectTicket` 和票据过期时间。

## HTTP 接口

### `POST /api/login/token`

请求体：

```json
{
  "account": "robot.10001",
  "token": "token-value"
}
```

处理顺序：

1. 校验 HTTP 方法、JSON 结构、`account` 和 `token`，并去除 `account` 首尾空白。
2. 调用 cache `CacheSetAccountVerifyToken`。
3. cache 使用 `account:{account}:token` 执行 `SETNX`，未消费 token 存在时返回失败。
4. 成功返回 `account`、`token` 和 `expireSecond`。

失败语义：

- `400`：请求结构或参数错误。
- `409`：同账号 token 已存在且未消费。
- `502/503/504`：cache 不可用或超时。

### `POST /api/login/session`

请求体：

```json
{
  "account": "robot.10001",
  "token": "token-value"
}
```

处理顺序：

1. 校验 HTTP 方法、JSON 结构、`account` 和 `token`，并去除 `account` 首尾空白。
2. 调用 cache `CacheUseAccountVerifyToken`，校验成功即消费 token。
3. 从 cache 返回可信 uid。
4. 从 gateway 注册表中选择 `availableLoad` 最大的 gateway。
5. 生成随机 nonce。
6. 构造 `connectTicket` payload：

```text
version, uid, account, gatewayKey, nonce, issuedAt, expireAt
```

7. 使用 `ticketSecret` 对 payload 做 HMAC-SHA256 签名。
8. 返回客户端连接 gateway 的信息。

成功响应：

```json
{
  "account": "robot.10001",
  "uid": 10001,
  "connectTicket": "...",
  "ticketExpireAt": 1717600000000,
  "gatewayKey": "/project/server/1/gateway/1/",
  "gatewayAddr": "192.168.71.123:10101"
}
```

失败语义：

- token 在 `CacheUseAccountVerifyToken` 校验成功后即标记为 used；如果后续账号创建、gateway 选择或票据签发失败，客户端需要重新申请 token。
- gateway 不可用时返回服务不可用，login 不直接创建 gateway session 的 Redis 状态。
- login 以 gRPC status code 作为 cache 错误权威，并通过公共映射转换为 HTTP status；`Aborted/AlreadyExists` 映射为冲突，`Unavailable` 映射为服务不可用，`DeadlineExceeded` 映射为网关超时。

## 数据流

```text
外部程序
  -> login /api/login/token
  -> cache CacheSetAccountVerifyToken
  -> Redis account:{account}:token

客户端
  -> login /api/login/session
  -> cache CacheUseAccountVerifyToken
  <- uid + gatewayAddr + connectTicket
  -> gateway UserVerifyReq(uid + connectTicket)
```

## 一致性约定

- login 不信任客户端提交的 uid，uid 只能来自 cache。
- login 不直接写用户在线态，在线态由 gateway 通过 cache CAS 维护。
- `connectTicket` 只用于客户端到指定 gateway 的首次 TCP 登录验证，过期后不能使用。
- `heartbeatSession` 不由 login 生成，也不进入 `connectTicket`；gateway 在 TCP 登录成功后本地生成并返回客户端。

## 排障

- `token not found or used`：token 不存在、已消费、已过期或 token 值不匹配。
- `gateway not available`：login 当前没有发现可用 gateway，检查 gateway etcd 注册和 `availableLoad`。
- `DeadlineExceeded`：cache RPC 超时，HTTP 边界返回 504。
- `connect ticket invalid`：检查 login/gateway 的 `ticketSecret`、gateway key 和时间是否一致。

## 后续建议

- 为 `POST /api/login/session` 补充更完整的 HTTP 边界错误码测试。
- 为 `POST /api/login/session` 增加覆盖 token 重放、ticket 过期/篡改、gateway 不可用的测试。
- 对 session 返回结果补充 trace id 或请求 id，方便跨 login、cache、gateway 日志串联。
