# Online 服务

Online 服务负责用户 actor、业务逻辑入口和 gateway stream 下行。当前在线 session 的写入、删除、续期和顶号编排已经迁到 gateway。部署、端口、容器启动和验证命令见 `deploy/online/README.md`。

## 能力边界

- 接收 gateway 的 `OnlineBindUser`，绑定 user actor。
- 接收 gateway 的 `OnlineUnbindUser`，按 `gatewayKey + userSession` 清理 actor。
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包。
- 通过 gateway stream 下发业务响应。
- 处理用户业务数据，例如 `UserRecordReq`、`UserCreateReq`、`RobotPingReq`。
- 绑定用户时从 cache 读取并校验 `UserRecord`。
- 更新 `UserRecord` 时调用 cache `CacheSetUserRecord`。

不再承担：

- 不查询或写入 `user:{uid}:session`。
- 不编排顶号。
- 不维护 session TTL。
- 不处理 `heartbeatSession` 轮换。
- 不维护 Redis session 中的 `onlineKey`；该字段由 gateway 写入，仅用于排障定位。

## OnlineBindUser

请求字段：

```text
uid
account
gatewayKey
clientIp
userSession
```

处理顺序：

1. 校验 uid、account、gateway key 和 `userSession` 非空。
2. 调用 cache `CacheGetUserRecord` 读取用户档案。
3. 校验 `UserRecord.uid/account` 与请求一致。
4. 按 uid 获取或创建 User actor。
5. User actor 绑定本地状态：`gatewayID`、`userSession`、account、clientIP、userRecord。
6. 写入 `GUserMgr.users[uid]`。
7. 返回 gateway。

gateway 在调用 `OnlineBindUser` 前已经完成 connectTicket 验签、旧连接顶号和 Redis session CAS；OnlineBindUser 内部读取并校验 UserRecord。
因此 online 不判断用户是否允许上线，也不创建抢占失败请求的 actor；只有已经抢到 Redis session 的 gateway 请求会进入 `OnlineBindUser`。
gateway 调用 `OnlineBindUser` 的默认超时时间为 `60s`。

## OnlineUnbindUser

请求字段：

```text
uid
gatewayKey
userSession
reason
msg
```

处理顺序：

1. 校验 uid、gateway key 和 `userSession` 非空。
2. 查找本地 User actor。
3. 本地 User 不存在时直接返回成功。
4. User actor 校验请求中的 gateway key 和 `userSession` 必须匹配本地状态。
5. 匹配时删除 `GUserMgr.users[uid]`，清空本地 gateway/session 状态并停止 actor。
6. 不匹配时忽略该解绑请求，防止旧请求误停新 actor。

Redis session 是否删除由 gateway 调用 `CacheEndUserSessionCAS` 决定。
gateway 调用 `OnlineUnbindUser` 的默认超时时间为 `60s`。

## 业务数据流

```text
client TCP
  -> gateway User actor
  -> gateway OnlineStreamTunnel client
  -> online OnlineStreamTunnel server
  -> online User actor
  -> gateway stream
  -> client TCP
```

当前已实现业务：

- `UserRecordReq`：返回 online 本地缓存的 `UserRecord`。
- `UserCreateReq`：设置 `user_create_time` 并调用 `CacheSetUserRecord`。
- `RobotPingReq`：返回 seq、clientTime、serverTime 和 payload。

## 一致性约定

- 同 uid 的 online 业务处理通过 User actor 串行执行。
- online actor 只接受匹配 `gatewayKey + userSession` 的解绑请求。
- online 不写 Redis session，因此不能作为“是否允许上线”的权威。
- `UserRecord` 由 online 登录绑定时从 cache 读取，online 业务更新时再写回 cache。

## 排障

- `user record mismatch`：online 从 cache 读取的 `UserRecord` 与 uid/account 不一致。
- `user not online` 不再作为解绑失败条件，本地 User 不存在会返回成功。
- 业务包无响应：检查 gateway stream 是否注册、online 是否有对应 uid actor。
- `DeadlineExceeded`：gateway 调用 online 超时，检查 gateway `onlineRPCTimeout`、online 日志和 actor 是否阻塞。

## 后续建议

- 补 `OnlineBindUser` actor 绑定测试和重复绑定测试。
- 补 `OnlineUnbindUser` 不存在成功、session 不匹配忽略、匹配停止 actor 测试。
- 将业务 handler 和登录 actor 状态拆分得更清晰，减少 online 主流程文件大小。
