# Online 服务

Online 服务负责用户 actor、业务逻辑入口和 gateway stream 下行。当前 cache userSession 的写入、删除、续期和顶号编排已经迁到 gateway。部署、端口、容器启动和验证命令见 `deploy/online/README.md`。

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
- 不维护 cache userSession TTL。
- 不处理 `heartbeatSession` 轮换。
- 不维护 cache userSession 中的 `onlineKey`; 该字段由 gateway 写入, 仅用于排障定位。

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

1. 校验 uid、account、gatewayKey 和 `userSession` 非空。
2. 调用 cache `CacheGetUserRecord` 读取用户档案。
3. 校验 `UserRecord.uid/account` 与请求一致。
4. 按 uid 获取或创建 User actor。
5. User actor 绑定本地状态: `gatewayKey`、`userSession`、account、clientIP、userRecord。
6. 写入 `GUserMgr.users[uid]`。
7. 返回 gateway。

gateway 在调用 `OnlineBindUser` 前已经完成 connectTicket 验签、旧连接顶号和 cache userSession CAS；OnlineBindUser 内部读取并校验 UserRecord。
因此 online 不判断用户是否允许上线，也不创建抢占失败请求的 actor；只有已经抢到 cache userSession 的 gateway 请求会进入 `OnlineBindUser`。
gateway 调用 `OnlineBindUser` 的默认超时时间以 `proto/online.grpc.proto` 中的 `methodOpt.timeout` 为准。

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

1. 校验 uid、gatewayKey 和 `userSession` 非空。
2. 查找本地 User actor。
3. 本地 User 不存在时直接返回成功。
4. User actor 校验请求中的 gatewayKey 和 `userSession` 必须匹配本地状态。
5. 匹配时删除 `GUserMgr.users[uid]`，清空本地 gatewayKey/userSession 状态并停止 actor。
6. 不匹配时忽略该解绑请求，防止旧请求误停新 actor。

cache userSession 是否删除由 gateway 调用 `CacheEndUserSessionCAS` 决定。
gateway 调用 `OnlineUnbindUser` 的默认超时时间以 `proto/online.grpc.proto` 中的 `methodOpt.timeout` 为准。

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
- `UserCreateReq`：在已绑定的 cache 档案上初始化服务端权威用户数据，设置 `user_create_timestamp_ms`，按 `used_uuid` 生成默认角色 `1000011 / 吉米` 和默认宠物记录，再调用 `CacheSetUserRecord` 写回; `uid/account/account_create_timestamp_ms` 必须来自 `OnlineBindUser` 绑定阶段已校验的 cache 档案。
- `RobotPingReq`：返回 seq、clientTime、serverTime 和 payload。

## 一致性约定

- 同 uid 的 online 业务处理通过 User actor 串行执行。
- online actor 只接受匹配 `gatewayKey + userSession` 的解绑请求。
- online 不写 cache userSession, 因此不能作为“是否允许上线”的权威。
- `UserRecord` 由 online 登录绑定时从 cache 读取，online 业务更新时再写回 cache。cache 只创建账号壳数据, 角色、宠物和后续业务数据由 online 初始化和维护。

## 排障

- `user record mismatch`：online 从 cache 读取的 `UserRecord` 与 uid/account 不一致。
- `user not online` 不再作为解绑失败条件，本地 User 不存在会返回成功。
- 业务包无响应：检查 gateway stream 是否注册、online 是否有对应 uid actor。
- `UserRecord` 缺少角色记录：检查客户端是否完成 `UserCreateReq`, 以及 online 写回 cache 是否成功。
- `DeadlineExceeded`：gateway 调用 online 超时，检查 gateway `onlineRPCTimeout`、online 日志和 actor 是否阻塞。

## 后续建议

- 补 `OnlineBindUser` actor 绑定测试和重复绑定测试。
- 补 `OnlineUnbindUser` 不存在成功、userSession 不匹配忽略、匹配停止 actor 测试。
- 将业务 handler 和登录 actor 状态拆分得更清晰，减少 online 主流程文件大小。
