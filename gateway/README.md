# Gateway 服务

Gateway 服务负责客户端 TCP 接入、首次登录验票、单登录顶号编排、在线 session CAS、心跳续期、离线清理和业务包透传。部署、端口、容器启动和验证命令见 `deploy/gateway/README.md`。

## 能力边界

- 监听客户端 TCP 长连接。
- 验证 `UserVerifyReq.uid + connectTicket`。
- 从 cache 读取 `user:{uid}:session`。
- 发现并调用旧 gateway `GatewayKickUser` 完成严格顶号。
- 通过 cache `CacheBeginUserSessionCAS` 抢占新在线 session。
- 选择可用 online，调用 `OnlineBindUser` 绑定 user actor。
- 维护本地 `heartbeatSession`，处理心跳轮换和 `CacheRefreshUserSessionCAS`。
- 在 TCP 断开、主动离线、心跳超时、顶号等场景调用 `OnlineUnbindUser` 和 `CacheEndUserSessionCAS`。
- gateway 到 cache、online 和旧 gateway 的 unary 超时统一由 proto `methodOpt.timeout` 控制；当前 cache 为 `3s`，online bind/unbind 和旧 gateway kick 为 `60s`。

## TCP 登录验证

客户端连接 gateway 后发送：

```text
UserVerifyReq {
  uid
  connectTicket
}
```

处理顺序：

1. 反序列化 `UserVerifyReq`。
2. 校验 uid 和 `connectTicket` 非空。
3. 使用 `ticketSecret` 验证 HMAC-SHA256 签名。
4. 校验票据未过期、payload uid 匹配、payload gatewayKey 等于当前 gateway key。
5. 从 payload 取得 account。
6. 生成固定 `userSession`。
7. 调用 `CacheGetUserSession` 查询旧在线态；不存在视为空 session。
8. 如果旧 session 存在，调用旧 gateway `GatewayKickUser(uid, oldUserSession)`；旧 gateway 不存在、找不到本地连接、`userSession` 不匹配或 cleanup 失败时，本次登录失败。
9. 选择 `availableLoad` 最大的 online。
10. 调用 `CacheBeginUserSessionCAS(expected_user_session="")` 写入带 `gatewayKey/userSession/login_time_ms/onlineKey` 的新 session；如果返回 `Aborted`，说明旧 session 仍存在或并发登录已抢占，本次登录失败。
11. 调用 `OnlineBindUser`，由 online 读取并校验 `UserRecord` 后绑定 actor。
12. 生成随机 `heartbeatSession`，绑定本地 User 到 uid、account、online、`userSession` 和 `heartbeatSession`。
13. 返回 `UserVerifyRes.server_time` 和 `UserVerifyRes.heartbeat_session`。

`OnlineBindUser` 只会在 `CacheBeginUserSessionCAS` 成功后调用。`CacheBeginUserSessionCAS` 失败时不会创建 online actor。

## 顶号流程

```text
new gateway
  -> cache CacheGetUserSession
  -> old gateway GatewayKickUser(uid, oldUserSession)
  -> old gateway 关闭旧 TCP
  -> old online OnlineUnbindUser(gatewayKey, oldUserSession)
  -> cache CacheEndUserSessionCAS(expected_user_session=oldUserSession)
  <- old gateway OK
  -> cache CacheBeginUserSessionCAS(expected_user_session="", new gatewayKey + userSession + login_time_ms + onlineKey)
  -> online OnlineBindUser
```

严格语义：

- 旧连接确认下线后，新连接才上线。
- 旧 gateway 不可达时不强制覆盖 Redis session。
- 旧 gateway 本地找不到连接时返回失败，等待 TTL 或运维清理。
- Redis CAS identity 固定为 `userSession`，防止旧请求误删新 session。
- 旧 gateway 返回成功后，新 gateway 不再二次读取 session，而是直接执行 `CacheBeginUserSessionCAS(expected_user_session="")`；CAS 冲突则失败关闭。
- 新 session 抢占成功后，如果 `OnlineBindUser`、本地 User 绑定或客户端连接状态检查失败，gateway 会调用 `OnlineUnbindUser` 和 `CacheEndUserSessionCAS(expected_user_session=userSession)` 回滚。

## 心跳

客户端心跳发送：

```text
UserHeartbeatReq.last_heartbeat_session
```

处理顺序：

1. gateway 校验客户端带回的 `last_heartbeat_session` 必须等于本地当前 `heartbeatSession`。
2. 不匹配视为重放、乱序或篡改，主动断开连接。
3. 生成随机 `next_heartbeat_session`。
4. 调用 `CacheRefreshUserSessionCAS(expected_user_session=userSession)` 刷新 Redis TTL。
5. cache 成功后更新本地 `heartbeatSession`。
6. 返回 `UserHeartbeatRes.next_heartbeat_session`。

`heartbeatSession` 只存在于客户端和 gateway 本地，不写入 Redis。

## 离线清理

离线来源：

- 客户端 TCP 主动关闭。
- 客户端发送 `UserOfflineReq`。
- 心跳超时。
- 新 gateway 调用 `GatewayKickUser`。
- gateway 本地异常导致连接关闭。

处理顺序：

1. gateway 从本地 UserMgr 删除 remote 和 uid 索引。
2. User actor 停止心跳/验证定时器。
3. 调用 online `OnlineUnbindUser(gatewayKey, userSession)` 清理 actor。
4. 调用 cache `CacheEndUserSessionCAS(expected_user_session=userSession)` 删除 Redis session。
5. `GatewayKickUser` 只有在旧 TCP 已断开、旧 online actor 已确认下线或不存在、Redis session 已 CAS 删除后才返回成功。

## 业务数据流

```text
client TCP
  -> gateway UserHandlerTCP
  -> gateway User actor
  -> online stream OnlineStreamTunnel
  -> online User actor
```

非登录包必须在 User 绑定 online 后才允许转发。未验证或 online 缺失时，gateway 会断开连接或返回错误。

## 一致性约定

- `user:{uid}:session` 由 gateway 写入、删除和续期。
- online 不再决定“谁能上线”，只管理 user actor。
- `userSession` 是固定连接身份，一次登录生成，心跳不轮换。
- `gatewayKey` 和 `onlineKey` 只作为 Redis session 元数据，分别用于定位旧 gateway 和排障定位 online。
- `heartbeatSession` 是客户端心跳凭证，可轮换，不进入 Redis。
- `connectTicket` 只负责首次 TCP 验证，不包含 `heartbeatSession`，不写 gateway pending 表，也不写 Redis。
- 所有 Redis session 写入、删除、续期都必须带 expected。
- `CacheGetUserSession` 对空 session 返回 `NotFound`。
- 当前不引入 Redis `binding` 状态。gateway 在 `CacheBeginUserSessionCAS` 成功后、`OnlineBindUser` 成功前崩溃时，Redis session 依赖 5 分钟 TTL 自然释放。

## 错误码和日志

- 服务间以 gRPC status code 为权威：CAS 冲突为 `Aborted`，票据或身份错误为 `Unauthenticated`，服务不可达为 `Unavailable`，超时为 `DeadlineExceeded`。
- TCP 边界通过公共映射转换为 `xerror` ResultID。
- 登录、顶号、回滚、心跳失败和 CAS 失败日志使用 `phase uid gatewayKey onlineKey userSession reason err` 这类 key=value 字段。
- `userSession` 和 `heartbeatSession` 只记录短前后缀，不打印完整凭证。
- 高频 TCP 每包日志为 Debug 级别，避免并发连接时日志放大。

## 排障

- `connectTicket invalid`：票据签名错误、过期、uid 不匹配或客户端连接了错误 gateway。
- `old gateway not found`：Redis session 指向的旧 gateway 当前未被 etcd 发现，新登录按严格语义失败。
- `heartbeatSession mismatch`：客户端心跳使用了旧 session、乱序 session 或被篡改的 session。
- `user session changed`：离线、顶号或 CAS 请求携带的 `userSession` 已不是当前连接。
- `DeadlineExceeded`：调用 cache、online 或旧 gateway 超时，检查 proto `methodOpt.timeout`、目标服务日志、网络和服务发现状态。

## 后续建议

- 收口 gateway gRPC 控制面暴露风险：`GatewayKickUser` 会断开用户连接并清理 session，当前部署示例会发布 `20101/20102` gRPC 端口且接口本身未做服务间鉴权。后续需要限制端口只对可信内网服务开放，并增加 mTLS 或 metadata token/HMAC 校验调用方身份。
- 增加顶号测试：旧 gateway 不存在、旧 gateway NotFound、kick 成功后 begin 成功、begin 失败回滚。
- 按压测结果继续调短 `online.grpc.proto` / `gateway.grpc.proto` 中 60 秒 `methodOpt.timeout`，避免异常场景连接长时间占用。
- CacheMgr 使用 xlib `MapMutexMgr` 缓存 cache 服务发现结果，由 etcd add/del 回调维护，并同步注册或摘除 gRPC resolve。
- User 心跳超时直接使用 `xtimer.Second` 维护；`heartbeatSession` 只负责客户端心跳凭证校验，不再使用 xlib `HeartBeat.WaitID`。
