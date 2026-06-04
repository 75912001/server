# Cache 服务

Cache 服务负责统一访问 Redis Cluster，提供账号登录 token、账号到 uid 映射、用户档案、在线 session 的 gRPC unary 接口。部署、端口、容器启动和验证命令见 `deploy/cache/README.md`。

## 能力边界

- 存储和消费账号级一次性登录 token。
- 确保账号存在，并为新账号分配 uid。
- 存储 `UserRecord`，Redis 中以 protobuf 二进制保存。
- 维护 `user:{uid}:session` 的批量读写、原子替换、删除和续期。
- 通过 Redis Lua 脚本实现 token 消费和 session CAS 操作，避免旧 online 的迟到请求污染新在线态。
- cache 只保存和校验数据，不决定用户应该归属哪个 gateway 或 online。

## 代码组织

- `main.go`：创建 cache 服务并执行 `PreStart`、`Start`、`PostStart`。
- `server.go`：初始化自定义配置、Redis Cluster、gRPC registry/selector，注册 `CacheService`，debug 模式注册 gRPC reflection。
- `cache.grpc.go`：cache gRPC server 类型。
- `cache.grpc.unary.account.token.go`：账号 token 写入、消费和账号创建入口。
- `cache.grpc.unary.user.record.go`：用户档案读写。
- `cache.grpc.unary.user.session.go`：在线 session 字段转换、批量读写、CAS 替换、删除和续期。
- `config.custom.go`：Redis key 格式、uid 起始值和账号创建锁时长。
- `redis.go`：`go-redis` ClusterClient 初始化、Ping、Close 和基础 Get。
- `redis.logic.account.token.go`：账号 token、账号到 uid 映射和账号创建锁。
- `redis.logic.user.record.go`：`UserRecord` protobuf 读写。
- `redis.logic.user.session.go`：用户在线 session hash、CAS Lua、TTL 和删除。
- `redis.logic.go`：Redis 逻辑公共 helper。
- `TEST.md`：cache 服务测试和构建命令。

## 启动流程

1. `NewCacheServer` 创建基础 `xserver.Server`，读取 custom 配置。
2. `PreStart` 初始化 gRPC proto registry 和 selector。
3. 使用 `xconfig.GConfigMgr.Redis` 创建 `redis.ClusterClient`。
4. 对 Redis 执行 `PING`，失败则启动失败。
5. 调用 `p.Server.PreStart`，由 xlib server 负责基础服务启动、etcd 上报和网络监听。
6. 注册 `cache.CacheService`。
7. `runMode=debug` 时注册 gRPC reflection，方便 `grpcurl list` 和 IDE 调试。

## 配置

Redis 配置来自 `bin/cache.yaml.template` 的 `redis` 项，当前使用 `redis.NewClusterClient`：

```yaml
redis:
  - name: cache
    addrs:
      - 192.168.71.123:7000
      - 192.168.71.123:7001
      - 192.168.71.123:7002
      - 192.168.71.123:7003
      - 192.168.71.123:7004
      - 192.168.71.123:7005
    password: "111111"
    dialTimeoutDuration: 3s
    readTimeoutDuration: 3s
    writeTimeoutDuration: 3s
```

custom 配置：

```yaml
custom:
  redisUIDSequenceSeed: 10000
  redisAccountCreateLockDuration: 5s
```

未显式配置时，代码使用以下默认 key 格式：

```text
redisKeyFormatUserRecord       user:{%v}:record
redisKeyFormatUserSession      user:{%v}:session
redisKeyFormatAccountToken     account:{%v}:token
redisKeyFormatAccountUID       account:{%v}:uid
redisKeyFormatAccountLock      account:{%v}:lock
redisKeyUserUIDSequence        user:uid:sequence
redisUIDSequenceSeed           10000
redisAccountCreateLockDuration 5s
```

`{...}` 是 Redis Cluster hash tag。`user:{uid}:record` 和 `user:{uid}:session` 会按同一个 uid 分到同一 slot；`account:{account}:token`、`uid`、`lock` 会按同一个 account 分到同一 slot。

## Redis Key

```text
account:{account}:token       一次性登录 token
account:{account}:uid         account 到 uid 的映射
account:{account}:lock        account 首次创建锁
user:uid:sequence             uid 自增序列
user:{uid}:record             UserRecord protobuf 二进制，默认实际格式为 user:{uid}:record
user:{uid}:session            在线 session hash，默认实际格式为 user:{uid}:session
```

`user:{uid}:session` hash 字段：

```text
gatewayKey
onlineKey
userSession
gatewaySession
loginTime
```

字段含义：

- `gatewayKey`：当前用户连接的 gateway 标识。
- `onlineKey`：当前维护用户在线态的 online 标识。
- `userSession`：一次登录生成的固定连接身份，心跳不轮换。
- `gatewaySession`：可轮换认证凭证，客户端心跳后更新。
- `loginTime`：登录时间。

## gRPC 接口

`CacheService` 使用 RingHash 负载策略，RPC 超时时间为 3 秒。

| RPC | shard key | 作用 |
| --- | --- | --- |
| `CacheSetAccountVerifyToken` | `account` | 写入账号级一次性 token，Redis 使用 `SETNX`，未消费前不覆盖。 |
| `CacheUseAccountVerifyToken` | `account` | 验证并消费 token，成功后确保账号存在并返回 uid。 |
| `CacheSetUserRecord` | `uid` | 写入 `UserRecord`，要求请求 `uid` 与 `UserRecord.uid` 一致。 |
| `CacheGetUserRecord` | `uid` | 读取 `UserRecord`。 |
| `CacheSetUserSessionRecord` | `uid` | 批量 HSET 在线 session 字段，不做 CAS，不设置 TTL。 |
| `CacheGetUserSessionRecord` | `uid` | 批量 HMGET 在线 session 字段，只返回 Redis 中存在的字段。 |
| `CacheReplaceUserSessionRecord` | `uid` | expected 匹配稳定 identity 后，原子替换完整 session，并按 `expire_second` 设置 TTL。 |
| `CacheSetUserSessionExpire` | `uid` | expected 匹配稳定 identity 后刷新 TTL，`expected_records` 必填。 |
| `CacheDelUserSessionRecord` | `uid` | expected 匹配稳定 identity 后删除 session。 |

## 错误语义

接口失败通过 gRPC status 返回：

| 场景 | code |
| --- | --- |
| 参数为空、uid 为 0、字段枚举非法、必填 records 缺失 | `InvalidArgument` |
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

`EnsureAccount` 处理顺序：

1. 查询 `account:{account}:uid`。
2. 已存在时读取 `user:{uid}:record` 并返回。
3. 不存在时获取 `account:{account}:lock`。
4. 拿到锁后再次查询账号映射，避免重复创建。
5. 初始化 `user:uid:sequence`，并通过 `INCR` 生成 uid。
6. 写入 `user:{uid}:record`，设置 `uid`、`account`、`account_create_time`，`user_create_time` 初始为 0。
7. 写入 `account:{account}:uid`。
8. 释放 `account:{account}:lock`。

如果 `account:{account}:uid` 已存在但 `user:{uid}:record` 缺失，cache 会按 account 和 uid 补建一个最小 `UserRecord`。如果旧 `UserRecord` 缺少 `uid`、`account`、`account_create_time`，读取账号时会补齐后写回。

保留 `account:{account}:lock` 的原因：

- 账号创建跨多个 Redis key，不是单条 Redis 原子操作。
- 没有锁时，并发请求可能生成多个 uid，只最终绑定其中一个，留下孤儿 `UserRecord`。
- 即使当前 token 消费会降低并发概率，锁仍是账号唯一性的最终保护。

## UserRecord

- `user:{uid}:record` 使用 protobuf marshal 后的二进制保存。
- `CacheSetUserRecord` 要求请求 `uid` 与 `UserRecord.uid` 完全一致，不在服务端自动补齐 `UserRecord.uid`。
- `CacheGetUserRecord` 对 Redis `nil` 返回 `NotFound`，其它 Redis 或反序列化错误返回 `Internal`。
- 直接在 Redis CLI 中看到 `\x08...` 属于正常现象。
- 读取时必须通过 `CacheGetUserRecord` 或 protobuf 反序列化解析。

## UserSession

`user:{uid}:session` 由 online 作为权威维护。cache 只提供 CAS 能力，不决定业务归属。

需要匹配的 identity 字段：

```text
gatewayKey + onlineKey + userSession
```

操作规则：

- `CacheSetUserSessionRecord`：直接 `HSET` 一个或多个字段，不做 CAS，不设置 TTL。调用方必须明确知道该写入允许覆盖。
- `CacheGetUserSessionRecord`：使用 `HMGET` 一次读取一个或多个字段；部分字段不存在时跳过，全部不存在时返回 `NotFound`。
- `CacheReplaceUserSessionRecord`：当前 Redis 中的 identity 与 expected 完全匹配时，替换为完整 session；`expire_second > 0` 时刷新 TTL。
- `CacheDelUserSessionRecord`：当前 Redis 中的 identity 与 expected 完全匹配时，删除 session。
- `CacheSetUserSessionExpire`：`expected_records` 必填，当前 Redis 中的 identity 与 expected 完全匹配时刷新 TTL。
- 心跳更新 `gatewaySession` 时，expected 会额外携带旧 `gatewaySession`，防止乱序心跳覆盖新值。
- 首次登录没有旧 session 时，online 仍会传入空值 identity 作为 expected；Lua 脚本将不存在的 hash 字段视为空字符串，从而只允许空在线态被写入。

必须匹配 session 的原因：

- 防止旧 online 的迟到删除误删新 online 写入的在线态。
- 防止并发登录互相覆盖。
- 防止旧 online 的迟到续期污染新 session。

## Redis 原子操作

- token 消费使用 Lua：`GET`、比较 token、`DEL` 在 Redis 内一次完成。
- session 替换使用 Lua：先校验 expected，再批量 `HSET` records，最后按需 `EXPIRE`。
- session 续期使用 Lua：先校验 expected，再执行 `EXPIRE`。
- session 删除使用 Lua：先校验 expected，再执行 `DEL`。
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

online
  -> CacheGetUserSessionRecord
  -> CacheReplaceUserSessionRecord
  -> CacheSetUserSessionExpire
  -> CacheDelUserSessionRecord
```

## 调试

debug 模式下 cache 注册 gRPC reflection：

```bash
grpcurl -plaintext localhost:20301 list
grpcurl -plaintext localhost:20301 list cache.CacheService
```

非 debug 模式下不注册 reflection，调试工具需要显式加载 `proto/cache.grpc.proto` 及其依赖 proto。

## 排障

- `token already exists`：同 account 已有未消费 token。
- `token not found or used`：token 不存在、过期、已消费或值不匹配。
- `user gatewaySession changed`：CAS expected 不匹配，说明在线态已被其他登录、离线或心跳更新接管。
- `user gatewaySession record not exist`：读取的 session 字段全部不存在。
- `redis: nil` 读取 `UserRecord`：用户档案缺失；如果是账号登录路径，`EnsureAccount` 会尝试补建最小档案。
- `redis addrs is empty`：`redis` 配置存在空地址列表。
- `redis config not found`：未配置 `redis` 项。

## 后续建议

- cache actor/worker 可按 shardKey 分发，让同 key 请求在 cache 进程内串行执行；这能减少锁竞争和乱序处理，但不能替代 Redis 锁和 CAS。
- 如需移除 `account:{account}:lock`，必须把账号创建改为 Redis Lua 原子流程，覆盖查询账号、生成 uid、写账号映射和写 UserRecord。
- 增加账号并发创建、token 重放、session CAS 冲突、旧删除迟到、新旧 session 乱序的自动化测试。
