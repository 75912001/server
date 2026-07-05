# Cache 服务

Cache 服务负责统一访问 Redis Cluster, 提供 accountVerifyToken、账号到 aid 映射、账号档案和 cache accountSession CAS gRPC 接口。部署、端口、容器启动和验证命令见 `deploy/cache/README.md`。

## 能力边界

- 存储和消费 accountVerifyToken。
- 确保账号存在，并为新账号分配 aid。
- 存储 `AccountRecord`，Redis 中以 protobuf 二进制保存。
- 维护 `account:{aid}:session` 对应 cache accountSession 的读取、开始、结束和 TTL 刷新 CAS。
- 通过 Redis Lua 脚本实现 accountVerifyToken 消费和 cache accountSession CAS 操作。
- cache 只保存和校验数据，不决定账号应该归属哪个 gateway 或 online。

## Redis Key

```text
account:{account}:accountVerifyToken accountVerifyToken
account:{account}:aid         account 到 aid 的映射
account:{account}:lock        account 首次创建锁
account:aid:sequence:{groupID}   当前 group 的 aid 自增序列
account:{aid}:record          AccountRecord protobuf 二进制
account:{aid}:session            cache accountSession hash
```

`{...}` 是 Redis Cluster hash tag。`account:{aid}:record` 和 `account:{aid}:session` 会按同一个 aid 分到同一 slot; `account:{account}:accountVerifyToken`、`aid`、`lock` 会按同一个 account 分到同一 slot。

## cache accountSession

`account:{aid}:session` 当前由 gateway 作为写入方维护。hash 字段：

```text
gatewayKey
accountSession
loginTimestampMs
onlineKey
```

字段含义：

- `gatewayKey`：当前账号连接的 gateway 标识，用于顶号时定位旧 gateway。
- `accountSession`：一次登录生成的固定连接身份，心跳不轮换。
- `loginTimestampMs`：Redis hash 字段名，表示登录时间毫秒值。
- `onlineKey`：当前绑定的 online 标识，只用于排障定位。

CAS identity 固定为：

```text
accountSession
```

`gatewayKey`、`onlineKey`、`loginTimestampMs` 都不参与 CAS 判断。`account:{aid}:session` 的 Redis key 已经限定 aid，因此 CAS 等价于 `aid + accountSession`。

`heartbeatSession` 不进入 Redis，只存在于客户端和 gateway 本地。

当前不保存 `state` 或 `binding` 字段。gateway 抢占 cache accountSession 后才会调用 online 绑定 actor; 如果绑定失败, gateway 负责删除 cache accountSession。若 gateway 在抢占成功后、绑定完成前崩溃, cache 不主动判定半成品状态, 该 cache accountSession 依赖 TTL 过期释放。

## gRPC 接口

`CacheService` 使用 RingHash 负载策略，gateway 调用这些 RPC 的默认超时时间为 3 秒。

| RPC | shard key | 作用 |
| --- | --- | --- |
| `CacheSetAccountVerifyToken` | `account` | 写入 accountVerifyToken, Redis 使用 `SETNX`, 未消费前不覆盖。 |
| `CacheUseAccountVerifyToken` | `account` | 验证并消费 accountVerifyToken, 成功后确保账号存在并返回 aid。 |
| `CacheSetAccountRecord` | `aid` | 写入 `AccountRecord`，要求请求 `aid` 与 `AccountRecord.aid` 一致。 |
| `CacheGetAccountRecord` | `aid` | 读取 `AccountRecord`。 |
| `CacheGetAccountSession` | `aid` | 读取当前 `gatewayKey/accountSession/loginTimestampMs/onlineKey`；`login_timestamp_ms` 对外表示登录时间毫秒值，读取不到完整 cache accountSession 时返回 `NotFound`。 |
| `CacheBeginAccountSessionCAS` | `aid` | `expected_account_session` 为空时要求当前 cache accountSession 不存在; 非空时要求当前 `accountSession` 匹配后替换为新 cache accountSession。 |
| `CacheEndAccountSessionCAS` | `aid` | `expected_account_session` 匹配时删除 cache accountSession。 |
| `CacheRefreshAccountSessionCAS` | `aid` | `expected_account_session` 匹配时刷新 cache accountSession TTL。 |

cache accountSession CAS 请求字段:

- `expected_account_session`: CAS 预期身份。begin 接口允许为空, 表示预期当前 cache accountSession 不存在; end/refresh 接口必须非空。
- `gateway_key`: begin 接口使用, gatewayKey, 用于定位当前 Gateway, 不能为空。
- `account_session`: begin 接口使用, 是新 cache accountSession 的稳定身份字段, 不能为空。
- `login_timestamp_ms`: begin 接口使用, 单位毫秒, 必须大于 0。
- `online_key`: begin 接口使用, onlineKey, 用于定位当前 Online, 不能为空。
- `expire_second`: begin/refresh 接口使用, 必须大于 0。

## 错误语义

| 场景 | code |
| --- | --- |
| 参数为空、aid 为 0、写入 cache accountSession 字段缺失、expire_second 为 0 | `InvalidArgument` |
| Redis 执行错误、序列化失败、账号数据异常 | `Internal` |
| accountVerifyToken 已存在 | `AlreadyExists` |
| accountVerifyToken 不存在、已使用或读取数据不存在 | `NotFound` |
| accountSession expected 不匹配 | `Aborted` |

## accountVerifyToken

`CacheSetAccountVerifyToken`：

1. 校验 `account`、`account_verify_token`、`expire_second`。
2. 对 `account:{account}:accountVerifyToken` 执行 `SETNX accountVerifyToken EX expire_second`。
3. key 已存在时返回 `AlreadyExists`, 不会覆盖旧 accountVerifyToken。

`CacheUseAccountVerifyToken`：

1. 校验 `account`、`account_verify_token`。
2. 用 Lua 原子读取 `account:{account}:accountVerifyToken`。
3. accountVerifyToken 不存在或不匹配时返回 `NotFound`。
4. accountVerifyToken 匹配时删除 accountVerifyToken key, 防止同一 accountVerifyToken 被重复消费。
5. 调用 `EnsureAccount`，返回可信 aid。

## 账号创建

AID 起始值由 cache 自身配置的 `base.groupID` 计算，公式位于 `common.GroupAIDStart`：

```text
GroupAIDStart(groupID) = uint64(groupID) * 1,000,000,000,000 + 1
```

`EnsureAccount` 处理顺序：

1. 查询 `account:{account}:aid`。
2. 已存在时读取 `account:{aid}:record` 并返回。
3. 不存在时获取 `account:{account}:lock`。
4. 拿到锁后再次查询账号映射，避免重复创建。
5. 初始化 `account:aid:sequence:{groupID}` 为 `GroupAIDStart(groupID)-1`，并通过 `INCR` 生成 aid。
6. 写入 `account:{aid}:record`，设置 `aid`、`account`、`account_create_timestamp_ms`，`account_record_create_timestamp_ms` 初始为 0；角色、宠物和其它游戏数据不在 cache 创建。
7. 写入 `account:{account}:aid`。
8. 释放 `account:{account}:lock`。

保留 `account:{account}:lock` 的原因：

- 账号创建跨多个 Redis key，不是单条 Redis 原子操作。
- 没有锁时，并发请求可能生成多个 aid，只最终绑定其中一个，留下孤儿 `AccountRecord`。
- 即使 accountVerifyToken 消费会降低并发概率, 锁仍是账号唯一性的最终保护。

## AccountRecord

- `account:{aid}:record` 使用 protobuf marshal 后的二进制保存。
- `AccountRecord` 是账号级档案聚合根, `aid/account` 下管理多个角色; `character_record_list` 的数组下标是角色槽位, 空槽使用 `uuid == 0` 的 `CharacterRecord` 占位, 每个账号最多可用角色槽位数量由 proto 常量 `AccountRecordLimit_MaxCharacterSlotCount` 定义, 完整角色业务 key 是 `aid + uuid`。
- `CharacterRecord.asset_id` 是角色资源 ID/角色 ID 的权威字段; `CharacterRecord.exp/earth/water/fire/wind/available_point/vitality/strength/toughness/dexterity/scene_id/create_timestamp_ms/rebirth_count/last_login_timestamp_ms/last_logout_timestamp_ms` 直接保存角色经验、元素点数、可用点、基础状态、当前场景、创建时间、转生次数和上下线时间; `asset_id_record_map` 当前不承载角色资源 ID、经验、元素、属性、场景、创建时间、转生次数、上下线时间戳、方向和动作。
- `CharacterRecord.pet_record_list` 只保存角色当前随身携带宠物, 按携带顺序排列, 单角色最多携带 `PetRecordLimit_MaxCarryCount` 只; `AccountRecord.pet_warehouse_record_map` 是账号宠物仓库, 同账号下所有角色共享, 最多存放 `AccountRecordLimit_MaxPetWarehouseCount` 只.
- `PetRecord.exp/loyalty/saved_base_*/raw_*/create_timestamp_ms/rebirth_count` 直接保存宠物经验、忠诚度、成长基础值、当前原始属性、创建时间和转生次数; `asset_record_base_map` 只保存宠物资源 ID。
- cache 只按 protobuf 透传存储 `AccountRecord`, 不校验宠物是否同时存在于角色随身携带列表和账号宠物仓库, 也不校验 `PetRecord.carry_status` 业务规则.
- `CacheSetAccountRecord` 要求请求 `aid` 与 `AccountRecord.aid` 完全一致。
- `CacheGetAccountRecord` 对 Redis `nil` 返回 `NotFound`，其它 Redis 或反序列化错误返回 `Internal`。
- `EnsureAccount` 只创建账号壳 `AccountRecord`; `used_uuid` 和 `character_record_list` 等游戏数据由 online 的 `CharacterCreateReq` 初始化后再通过 `CacheSetAccountRecord` 写回。
- 本轮不兼容旧 cache `AccountRecord`; 已存在但缺少 `CharacterRecord.asset_id/vitality` 等角色根字段的档案视为旧格式, 开发环境需要清理 cache 或重新创建账号。
- 直接在 Redis CLI 中看到 `\x08...` 属于正常现象。
- 读取时必须通过 `CacheGetAccountRecord` 或 protobuf 反序列化解析。

## Redis 原子操作

- accountVerifyToken 消费使用 Lua: `GET`、比较 accountVerifyToken、`DEL` 在 Redis 内一次完成。
- cache accountSession begin 使用 Lua：expected 为空时检查 key 不存在；expected 非空时校验 identity，再写入完整 cache accountSession 并设置 TTL。
- cache accountSession end 使用 Lua：校验 expected identity 后执行 `DEL`。
- cache accountSession refresh 使用 Lua：校验 expected identity 后执行 `EXPIRE`。
- Lua 脚本返回 `1` 表示成功, 返回 `0` 表示 accountVerifyToken 不匹配、accountSession 不匹配或 key 不存在。

## 数据流

```text
login
  -> CacheSetAccountVerifyToken
  -> Redis account:{account}:accountVerifyToken

login
  -> CacheUseAccountVerifyToken
  -> Redis account:{account}:accountVerifyToken
  -> EnsureAccount
  -> Redis account:{account}:aid
  -> Redis account:{aid}:record

login email/password
  -> CacheSetAccountVerifyToken
  -> Redis account:{account}:accountVerifyToken
  -> CacheUseAccountVerifyToken
  -> Redis account:{account}:accountVerifyToken
  -> EnsureAccount
  -> Redis account:{account}:aid
  -> Redis account:{aid}:record

gateway
  -> CacheGetAccountRecord
  -> CacheGetAccountSession
  -> CacheBeginAccountSessionCAS
  -> CacheRefreshAccountSessionCAS
  -> CacheEndAccountSessionCAS

online
  -> CacheSetAccountRecord
```

## 排障

- `accountVerifyToken already exists`: 同 account 已有未消费 accountVerifyToken。
- `accountVerifyToken not found or used`: accountVerifyToken 不存在、过期、已消费或值不匹配。
- `account session changed`：CAS expected 不匹配，说明 cache accountSession 已被其他登录、离线或 TTL 变化接管。
- `account session not found`：当前 aid 没有 cache accountSession。
- `redis: nil` 读取 `AccountRecord`：账号档案缺失；如果账号映射已存在，`EnsureAccount` 会按账号数据不一致返回错误。
- `redis addrs is empty`：`redis` 配置存在空地址列表。
- `redis config not found`：未配置 `redis` 项。

## 后续建议

- cache actor/worker 可按 shardKey 分发，让同 key 请求在 cache 进程内串行执行；这能减少锁竞争和乱序处理，但不能替代 Redis 锁和 CAS。
- 如需移除 `account:{account}:lock`，必须把账号创建改为 Redis Lua 原子流程，覆盖查询账号、生成 aid、写账号映射和写 AccountRecord。
- 增加账号并发创建、accountVerifyToken 重放、cache accountSession CAS 冲突、旧删除迟到、新旧 cache accountSession 乱序的自动化测试。
