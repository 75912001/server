# Cache 服务

Cache 服务负责统一访问 Redis Cluster，提供账号登录 token、账号到 uid 映射、用户档案和在线 session CAS gRPC 接口。部署、端口、容器启动和验证命令见 `deploy/cache/README.md`。

## 能力边界

- 存储和消费账号级一次性登录 token。
- 确保账号存在，并为新账号分配 uid。
- 存储 `UserRecord`，Redis 中以 protobuf 二进制保存。
- 维护 `user:{uid}:session` 的读取、开始、结束和 TTL 刷新 CAS。
- 通过 Redis Lua 脚本实现 token 消费和 session CAS 操作。
- cache 只保存和校验数据，不决定用户应该归属哪个 gateway 或 online。

## Redis Key

```text
account:{account}:token       一次性登录 token
account:{account}:uid         account 到 uid 的映射
account:{account}:lock        account 首次创建锁
user:uid:sequence:{groupID}   当前 group 的 uid 自增序列
user:{uid}:record             UserRecord protobuf 二进制
user:{uid}:session            在线 session hash
```

`{...}` 是 Redis Cluster hash tag。`user:{uid}:record` 和 `user:{uid}:session` 会按同一个 uid 分到同一 slot；`account:{account}:token`、`uid`、`lock` 会按同一个 account 分到同一 slot。

## UserSession

`user:{uid}:session` 当前由 gateway 作为写入方维护。hash 字段：

```text
gatewayKey
userSession
loginTime
onlineKey
```

字段含义：

- `gatewayKey`：当前用户连接的 gateway 标识，用于顶号时定位旧 gateway。
- `userSession`：一次登录生成的固定连接身份，心跳不轮换。
- `loginTime`：Redis hash 字段名，表示登录时间毫秒值。
- `onlineKey`：当前绑定的 online 标识，只用于排障定位。

CAS identity 固定为：

```text
userSession
```

`gatewayKey`、`onlineKey`、`loginTime` 都不参与 CAS 判断。`user:{uid}:session` 的 Redis key 已经限定 uid，因此 CAS 等价于 `uid + userSession`。

`heartbeatSession` 不进入 Redis，只存在于客户端和 gateway 本地。

当前不保存 `state` 或 `binding` 字段。gateway 抢占 session 后才会调用 online 绑定 actor；如果绑定失败，gateway 负责删除 session。若 gateway 在抢占成功后、绑定完成前崩溃，cache 不主动判定半成品状态，该 session 依赖 TTL 过期释放。

## gRPC 接口

`CacheService` 使用 RingHash 负载策略，gateway 调用这些 RPC 的默认超时时间为 3 秒。

| RPC | shard key | 作用 |
| --- | --- | --- |
| `CacheSetAccountVerifyToken` | `account` | 写入账号级一次性 token，Redis 使用 `SETNX`，未消费前不覆盖。 |
| `CacheUseAccountVerifyToken` | `account` | 验证并消费 token，成功后确保账号存在并返回 uid。 |
| `CacheSetUserRecord` | `uid` | 写入 `UserRecord`，要求请求 `uid` 与 `UserRecord.uid` 一致。 |
| `CacheGetUserRecord` | `uid` | 读取 `UserRecord`。 |
| `CacheGetUserSession` | `uid` | 读取当前 `gatewayKey/userSession/loginTime/onlineKey`；`login_time_ms` 对外表示登录时间毫秒值，读取不到完整 session 时返回 `NotFound`。 |
| `CacheBeginUserSessionCAS` | `uid` | `expected_user_session` 为空时要求当前 session 不存在；非空时要求当前 `userSession` 匹配后替换为新 session。 |
| `CacheEndUserSessionCAS` | `uid` | `expected_user_session` 匹配时删除 session。 |
| `CacheRefreshUserSessionCAS` | `uid` | `expected_user_session` 匹配时刷新 session TTL。 |

Session CAS 请求字段：

- `expected_user_session`：CAS 预期身份。begin 接口允许为空，表示预期当前 session 不存在；end/refresh 接口必须非空。
- `gateway_key`：begin 接口使用，用于定位当前 gateway，不能为空。
- `user_session`：begin 接口使用，是新在线会话的稳定身份字段，不能为空。
- `login_time_ms`：begin 接口使用，单位毫秒，必须大于 0。
- `online_key`：begin 接口使用，用于定位当前 online，不能为空。
- `expire_second`：begin/refresh 接口使用，必须大于 0。

## 错误语义

| 场景 | code |
| --- | --- |
| 参数为空、uid 为 0、写入 session 字段缺失、expire_second 为 0 | `InvalidArgument` |
| Redis 执行错误、序列化失败、账号数据异常 | `Internal` |
| token 已存在 | `AlreadyExists` |
| token 不存在、已使用或读取数据不存在 | `NotFound` |
| session expected 不匹配 | `Aborted` |

## Account Token

`CacheSetAccountVerifyToken`：

1. 校验 `account`、`token`、`expire_second`。
2. 对 `account:{account}:token` 执行 `SETNX token EX expire_second`。
3. key 已存在时返回 `AlreadyExists`，不会覆盖旧 token。

`CacheUseAccountVerifyToken`：

1. 校验 `account`、`token`。
2. 用 Lua 原子读取 `account:{account}:token`。
3. token 不存在或不匹配时返回 `NotFound`。
4. token 匹配时删除 token key，防止同一 token 被重复消费。
5. 调用 `EnsureAccount`，返回可信 uid。

## 账号创建

UID 起始值由 cache 自身配置的 `base.groupID` 计算，公式位于 `common.GroupUIDStart`：

```text
GroupUIDStart(groupID) = uint64(groupID) * 1,000,000,000,000 + 1
```

`EnsureAccount` 处理顺序：

1. 查询 `account:{account}:uid`。
2. 已存在时读取 `user:{uid}:record` 并返回。
3. 不存在时获取 `account:{account}:lock`。
4. 拿到锁后再次查询账号映射，避免重复创建。
5. 初始化 `user:uid:sequence:{groupID}` 为 `GroupUIDStart(groupID)-1`，并通过 `INCR` 生成 uid。
6. 写入 `user:{uid}:record`，设置 `uid`、`account`、`account_create_time`，`user_create_time` 初始为 0。
7. 写入 `account:{account}:uid`。
8. 释放 `account:{account}:lock`。

保留 `account:{account}:lock` 的原因：

- 账号创建跨多个 Redis key，不是单条 Redis 原子操作。
- 没有锁时，并发请求可能生成多个 uid，只最终绑定其中一个，留下孤儿 `UserRecord`。
- 即使 token 消费会降低并发概率，锁仍是账号唯一性的最终保护。

## UserRecord

- `user:{uid}:record` 使用 protobuf marshal 后的二进制保存。
- `CacheSetUserRecord` 要求请求 `uid` 与 `UserRecord.uid` 完全一致。
- `CacheGetUserRecord` 对 Redis `nil` 返回 `NotFound`，其它 Redis 或反序列化错误返回 `Internal`。
- 直接在 Redis CLI 中看到 `\x08...` 属于正常现象。
- 读取时必须通过 `CacheGetUserRecord` 或 protobuf 反序列化解析。

## Redis 原子操作

- token 消费使用 Lua：`GET`、比较 token、`DEL` 在 Redis 内一次完成。
- session begin 使用 Lua：expected 为空时检查 key 不存在；expected 非空时校验 identity，再写入完整 session 并设置 TTL。
- session end 使用 Lua：校验 expected identity 后执行 `DEL`。
- session refresh 使用 Lua：校验 expected identity 后执行 `EXPIRE`。
- Lua 脚本返回 `1` 表示成功，返回 `0` 表示 token 不匹配、session 不匹配或 key 不存在。

## 数据流

```text
login
  -> CacheSetAccountVerifyToken
  -> Redis account:{account}:token

login
  -> CacheUseAccountVerifyToken
  -> Redis account:{account}:token
  -> EnsureAccount
  -> Redis account:{account}:uid
  -> Redis user:{uid}:record

gateway
  -> CacheGetUserRecord
  -> CacheGetUserSession
  -> CacheBeginUserSessionCAS
  -> CacheRefreshUserSessionCAS
  -> CacheEndUserSessionCAS

online
  -> CacheSetUserRecord
```

## 排障

- `token already exists`：同 account 已有未消费 token。
- `token not found or used`：token 不存在、过期、已消费或值不匹配。
- `user session changed`：CAS expected 不匹配，说明在线态已被其他登录、离线或 TTL 变化接管。
- `user session not found`：当前 uid 没有在线 session。
- `redis: nil` 读取 `UserRecord`：用户档案缺失；如果账号映射已存在，`EnsureAccount` 会按账号数据不一致返回错误。
- `redis addrs is empty`：`redis` 配置存在空地址列表。
- `redis config not found`：未配置 `redis` 项。

## 后续建议

- cache actor/worker 可按 shardKey 分发，让同 key 请求在 cache 进程内串行执行；这能减少锁竞争和乱序处理，但不能替代 Redis 锁和 CAS。
- 如需移除 `account:{account}:lock`，必须把账号创建改为 Redis Lua 原子流程，覆盖查询账号、生成 uid、写账号映射和写 UserRecord。
- 增加账号并发创建、token 重放、session CAS 冲突、旧删除迟到、新旧 session 乱序的自动化测试。
