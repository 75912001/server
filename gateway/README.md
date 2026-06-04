# Gateway 服务

Gateway 服务负责客户端 TCP 接入、登录验证、连接状态维护、业务包透传、心跳和离线清理。部署、端口、容器启动和验证命令见 `deploy/gateway/README.md`。

## 能力边界

- 监听客户端 TCP 连接。
- 接收 login 的 `GatewayPrepareLogin`，在本地保存 pending login session。
- 校验客户端 `UserVerifyReq` 中的 uid、`gatewaySession` 和 `gatewayNonce`。
- 选择可用 online 并调用 `OnlineUserOnline`。
- 登录成功后绑定本地 User、固定 `userSession`、启动心跳超时计时器，并通过 stream 向 online 透传业务包。
- 处理心跳，轮换 `gatewaySession`。
- 在 TCP 断开、主动离线、顶号等场景通知 online 清理 session。

## 登录准备

`GatewayPrepareLogin` 来自 login，字段包括：

```text
uid
account
gatewayNonce
gatewaySession
expireSecond
```

处理顺序：

1. 校验字段非空。
2. 使用当前 gateway etcd key 重新计算 `gatewaySession`。
3. 计算公式：

```text
sha256(uid + ":" + gatewayKey + ":" + gatewayNonce + ":" + "menglc-session")
```

4. 校验通过后写入本地 pending 表。
5. pending 到期未被客户端验证时自动删除。

pending 是 gateway 本地状态，不写 Redis。它只用于桥接 login HTTP 返回和客户端 TCP 登录验证。

## TCP 登录验证

客户端连接 gateway 后发送 `UserVerifyReq`：

```text
uid
gatewaySession
gatewayNonce
```

处理顺序：

1. 反序列化 `UserVerifyReq`。
2. 校验 uid、`gatewayNonce`、`gatewaySession` 非空。
3. 使用当前 gateway key 重新计算并比较 `gatewaySession`。
4. 消费本地 pending login session；同一个 pending 只能消费一次。
5. 选择 `availableLoad` 最大的 online。
6. 生成固定 `userSession`，调用 online `OnlineUserOnline`。
7. online 成功后绑定本地 User 到 uid、account、online、`userSession` 和 `gatewaySession`。
8. 返回 `UserVerifyRes`。

验证失败会返回对应 ResultID，例如 session 不匹配、pending 不存在、online 不可用或 RPC 超时。

## 业务数据流

```text
client TCP
  -> gateway UserHandlerTCP
  -> gateway User actor
  -> online stream OnlineStreamTunnel
  -> online User actor
```

非登录包必须在 User 绑定 online 后才允许转发。未验证或 online 缺失时，gateway 会断开连接或返回错误。

## 心跳和 gatewaySession 轮换

客户端心跳发送 `UserHeartbeatReq.last_gateway_session`。

处理顺序：

1. gateway 校验客户端带回的 `last_gateway_session` 必须等于本地当前 `gatewaySession`。
2. 校验失败视为重放、乱序或篡改，gateway 主动断开连接。
3. gateway 生成随机 `next_gateway_session`。
4. 调用 online `OnlineUserUpdateGatewaySession`，请求携带固定 `userSession` 和旧 `gatewaySession`。
5. online 成功替换 Redis session 后，gateway 更新本地 `gatewaySession`。
6. 返回 `UserHeartbeatRes.next_gateway_session`。

## 离线清理

离线来源：

- 客户端 TCP 主动关闭。
- 客户端发送 `UserOfflineReq`。
- 心跳超时。
- online 顶号时调用 `GatewayUserOffline`。
- gateway 本地异常导致连接关闭。

处理顺序：

1. gateway 从本地 UserMgr 删除 remote 和 uid 索引。
2. User actor 执行 cleanup。
3. 若 uid、online、`userSession`、`gatewaySession` 有效，则调用 online `OnlineUserOffline`。
4. online 按 expected session 删除 Redis 在线态。

## 一致性约定

- gateway 不直接写 `user:{uid}:session`，在线态由 online 作为权威维护。
- gateway 本地 `userSession` 是固定连接身份，顶号和离线只处理匹配的连接。
- gateway 本地 `gatewaySession` 必须与 online/cache 中的 session 同步更新。
- pending login session 一次性消费，过期后不能用于登录。
- `GatewayUserOffline` 只断开 `userSession` 匹配的本地连接；Redis session 是否删除由旧 online 的 expected session 决定。

## 排障

- `gatewaySession mismatch`：客户端提交的 uid、gateway key、nonce 或 session 不匹配。
- `pending session not found`：pending 已过期、已消费、gateway 重启或客户端连接了错误 gateway。
- `DeadlineExceeded`：调用 online 登录或更新 session 超时。当前 X gRPC 连接有超时拦截器，后续建议在调用点显式 `context.WithTimeout`。
- `packet from unknown remote`：连接没有成功绑定 User 或连接已清理后仍收到包。
- `heartbeat gatewaySession mismatch`：客户端心跳使用了旧 session、乱序 session 或被篡改的 session。
- `user session changed`：离线或顶号请求携带的 `userSession` 已不是当前连接。

## 后续建议

- 为 gateway 到 online 的登录、离线、session 更新调用显式设置超时和日志字段。
- 记录 `uid`、`gatewayKey`、`onlineKey`、`userSession`、`gatewaySession` 的短 hash，便于跨服务排查。
- 增加 pending 过期、错误 gateway、心跳乱序、顶号断开和 online 超时的测试。

## 实现细节补充

- pending login session 使用 xlib `MapMutexMgr` 存储，单次 `Add/Find/Del` 由 map 内部锁保护。
- pending login session 过期使用 xlib 全局 `GTimer`，到期事件投递到 gateway 主 actor 后执行删除。
- pending 不保存 timer 句柄，登录验证 `Consume` 只删除 map key，不调用 `DelSecond`，避免 unary 路径与 timer 状态竞争。
- 每次登录准备都会生成新的 `gatewayNonce/gatewaySession`，同一个 `loginSessionKey(uid, gatewaySession)` 不预期重复，`Add` 直接写入当前 pending。
- `Expire` 只按 key 删除；如果 pending 已被 `Consume` 删除，过期事件晚到时就是 no-op。
