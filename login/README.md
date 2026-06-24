# Login 服务

Login 服务是账号登录链路的 HTTP 入口，负责接收外部程序写入的 accountVerifyToken，也支持客户端直接使用 email/password 登录，并为登录成功的客户端选择 gateway、签发短期 `connectTicket`。部署、端口、容器启动和验证命令见 `deploy/login/README.md`。

## 服务设计

login 只做登录凭证校验和 gateway 入口分配，不维护用户在线态。

- 外部程序通过 `POST /api/login/accountVerifyToken` 写入 `account/accountVerifyToken`。
- 客户端通过 `POST /api/login/session` 使用 `account/accountVerifyToken` 换取 `uid/connectTicket/gateway`。
- 客户端也可以通过 `POST /api/login/emailSession` 使用 `email/password` 换取 `uid/connectTicket/gateway`。
- email/password 用户列表来自 login 当前运行配置文件的 `custom.emailPasswordUsers`，每次 email 登录都会重新读取文件。
- `uid` 只能由 cache 根据账号解析或创建，login 不接受客户端提交 uid。
- gateway 由 login 从 etcd 发现，按本地 `availableLoad` 最大优先选择；负载相同时按 gateway key 字典序稳定选择。
- login 选中 gateway 后会先本地扣减 1 个 `availableLoad`，避免短时间批量请求全部打到同一个实例；后续 etcd update 会覆盖本地估算值。
- `connectTicket` 由 login 使用 HMAC-SHA256 签发，绑定 `uid/account/gatewayKey/nonce/expireTimestampMs`。
- 客户端连接 gateway 后发送 `UserVerifyReq(uid, connect_ticket)`，gateway 负责验签和后续 online/cache 在线流程。

## 架构

```text
外部程序
  -> login HTTP /api/login/accountVerifyToken
  -> cache CacheSetAccountVerifyToken
  -> Redis account:{account}:accountVerifyToken

客户端
  -> login HTTP /api/login/session
  -> cache CacheUseAccountVerifyToken
  <- uid + connectTicket + gatewayKey + gatewayAddr
  -> gateway TCP UserVerifyReq(uid + connectTicket)
  -> online/cache 用户在线流程

客户端
  -> login HTTP /api/login/emailSession
  -> login 读取运行配置 custom.emailPasswordUsers
  -> cache CacheSetAccountVerifyToken
  -> cache CacheUseAccountVerifyToken
  <- uid + connectTicket + gatewayKey + gatewayAddr
  -> gateway TCP UserVerifyReq(uid + connectTicket)
```

## 核心能力

- HTTP 服务使用标准库 `net/http`，由 `xserver.Server` 托管生命周期。
- cache 节点通过 etcd add/del 事件维护，并同步注册到 xlib gRPC resolver，HTTP 请求调用 `pb.GXCacheServiceService`。
- gateway 节点通过 etcd add/update/del 事件维护，update 会用 etcd 权威值覆盖 gateway 地址、负载和实例信息。
- HTTP 请求体使用 `json.Decoder.DisallowUnknownFields`，拒绝未知字段。
- 请求体大小受 `maxBodyBytes` 限制，避免外部 HTTP 请求无限读入。
- cache RPC 受 `cacheRPCTimeout` 控制，HTTP 关闭受 `shutdownTimeout` 控制。

## HTTP 接口

### `POST /api/login/accountVerifyToken`

外部程序调用，用于写入账号级一次性验证凭证 accountVerifyToken。该接口不创建 uid，不选择 gateway，不返回 connectTicket。

请求体：

```json
{
  "account": "robot.10001",
  "accountVerifyToken": "account-verify-token-value"
}
```

成功响应：

```json
{
  "account": "robot.10001",
  "accountVerifyToken": "account-verify-token-value",
  "expireSecond": 10
}
```

处理顺序：

1. 要求 HTTP method 为 `POST`。
2. 解析 JSON，请求只允许 `account` 和 `accountVerifyToken`。
3. 对 `account` 去除首尾空格，空 `account/accountVerifyToken` 返回 `400`。
4. 调用 cache `CacheSetAccountVerifyToken(account, accountVerifyToken, expireSecond)`。
5. cache 使用账号级 accountVerifyToken key 写入一次性验证凭证，未消费 accountVerifyToken 存在时返回冲突。

### `POST /api/login/session`

客户端调用，用于消费 accountVerifyToken 并换取连接 gateway 所需的信息。

请求体：

```json
{
  "account": "robot.10001",
  "accountVerifyToken": "account-verify-token-value"
}
```

成功响应：

```json
{
  "account": "robot.10001",
  "uid": 10001,
  "connectTicket": "...",
  "ticketExpireTimestampMs": 1717600000000,
  "gatewayKey": "/project/server/1/gateway/1/",
  "gatewayAddr": "192.168.71.123:10101"
}
```

处理顺序：

1. 要求 HTTP method 为 `POST`。
2. 解析 JSON，请求只允许 `account` 和 `accountVerifyToken`。
3. 对 `account` 去除首尾空格，空 `account/accountVerifyToken` 返回 `400`。
4. 调用 cache `CacheUseAccountVerifyToken(account, accountVerifyToken)`，由 cache 原子验证并消费 accountVerifyToken。
5. cache 消费成功后确保账号存在，必要时创建 `account -> uid` 映射和占位 `AccountRecord`，并返回可信 uid。
6. login 从 gateway 管理器中选择本地 `availableLoad` 最大的可用 gateway，并先本地扣减 1 个可用负载。
7. login 构造 `ConnectTicketPayload`，payload 包含 `version/uid/account/gatewayKey/nonce/issuedTimestampMs/expireTimestampMs`。
8. login 使用 `ticketSecret` 对 payload 做 HMAC-SHA256 签名，返回 `connectTicket` 和目标 gateway 信息。

### `POST /api/login/emailSession`

客户端调用，用于使用 email/password 换取连接 gateway 所需的信息。email trim 后统一转小写，并作为 cache 中的 `account`。

请求体：

```json
{
  "email": "user@example.com",
  "password": "plain-password"
}
```

成功响应：

```json
{
  "account": "user@example.com",
  "uid": 10001,
  "connectTicket": "...",
  "ticketExpireTimestampMs": 1717600000000,
  "gatewayKey": "/project/server/1/gateway/1/",
  "gatewayAddr": "192.168.71.123:10101"
}
```

处理顺序：

1. 要求 HTTP method 为 `POST`。
2. 解析 JSON，请求只允许 `email` 和 `password`。
3. `email` 去除首尾空格后转小写，`password` 不 trim，按原字符串精确匹配。
4. 每次请求重新读取 login 当前运行配置文件，解析 `custom.emailPasswordUsers`。
5. 配置中 email 同样 trim 后转小写；email 不存在或密码错误返回 `401`。
6. login 生成内部随机 accountVerifyToken，调用 cache `CacheSetAccountVerifyToken(account=email, accountVerifyToken, expireSecond)`。
7. login 立即调用 cache `CacheUseAccountVerifyToken(account=email, accountVerifyToken)`，由 cache 原子消费 accountVerifyToken、确保账号存在并返回可信 uid。
8. login 选择 gateway、签发 `connectTicket`，返回连接信息。

## 错误语义

- `400`：HTTP 方法、JSON 结构、未知字段、空 `account/accountVerifyToken` 等请求错误。
- `401`：email/password 登录时邮箱不存在或密码错误。
- `409`：同账号 accountVerifyToken 已存在且未消费，或 cache 返回冲突类状态。
- `500`：email/password 配置读取失败、YAML 解析失败、重复 email、空 email/password，或签发票据失败。
- `502`：cache 返回内部错误或数据异常。
- `503`：cache/gateway 不可用。
- `504`：cache RPC 超时。

accountVerifyToken 在 `CacheUseAccountVerifyToken` 成功后即被消费；如果后续没有可用 gateway 或签发 `connectTicket` 失败，login 不回滚 accountVerifyToken，客户端需要重新申请 accountVerifyToken。

## 配置

当前 `bin/login.yaml` 只必须显式配置 `custom.httpAddr`，其它 custom 配置都有代码默认值，需要改默认行为时再写入配置文件。

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `custom.httpAddr` | 无 | login HTTP 监听地址，必须配置 |
| `custom.accountVerifyTokenPath` | `/api/login/accountVerifyToken` | 外部程序写 accountVerifyToken 的 HTTP 路径 |
| `custom.sessionPath` | `/api/login/session` | 客户端换取登录票据的 HTTP 路径 |
| `custom.emailSessionPath` | `/api/login/emailSession` | 客户端使用 email/password 换取登录票据的 HTTP 路径 |
| `custom.emailPasswordUsers` | `[]` | email/password 用户列表；注册系统写入当前运行配置文件后，下次 email 登录读取生效 |
| `custom.accountVerifyTokenExpireSecond` | `10s` | accountVerifyToken 有效时间，配置使用 Go duration 字符串 |
| `custom.ticketExpireSecond` | `10s` | `connectTicket` 有效时间，配置使用 Go duration 字符串 |
| `custom.ticketSecret` | `common.ConnectTicketSecretDefault` | `connectTicket` HMAC 密钥，必须和 gateway 一致 |
| `custom.readHeaderTimeout` | `5s` | HTTP 读取请求头超时时间 |
| `custom.shutdownTimeout` | `10s` | HTTP 优雅关闭等待时间 |
| `custom.cacheRPCTimeout` | `3s` | login 调用 cache RPC 的超时时间 |
| `custom.maxBodyBytes` | `4096` | HTTP 请求体最大字节数 |

## etcd 发现

- add cache：先移除同 key 旧 cache，再创建新连接并注册到 xlib resolver。
- del cache：从本地索引和 xlib resolver 移除，并关闭连接。
- add gateway：先移除同 key 旧 gateway，再创建新连接。
- update gateway：用 etcd 权威值更新 `availableLoad`、TCP 地址、gRPC 地址和实例信息；gRPC 地址不变时复用旧连接。
- del gateway：从本地索引移除，并关闭连接。

login 的 etcd 回调按顺序执行，cache 本地 map 不额外加锁；gateway 管理器使用 xlib 的带锁 map，便于 HTTP 请求并发读取。

## 一致性约定

- login 不信任客户端 uid，也不接受 uid 入参。
- `/api/login/accountVerifyToken` 只写 accountVerifyToken，不创建账号映射和用户档案。
- `/api/login/session` 只在 accountVerifyToken 消费成功后才获取 uid 和签发票据。
- `/api/login/emailSession` 在 email/password 校验成功后生成内部随机 accountVerifyToken，并复用 `CacheSetAccountVerifyToken`、`CacheUseAccountVerifyToken` 完成账号创建和 uid 获取。
- email/password 明文保存在 login 当前运行配置文件中；生产环境需要通过文件权限和部署流程保护该配置。
- login 不调用 gateway prepare-login，不写 gateway pending 表，也不写 Redis 在线态。
- `connectTicket` 只负责首次连接 gateway 的身份校验；登录成功后的 `heartbeatSession` 由 gateway 生成和轮换，不进入 login。
- `ticketSecret` 是 login 和 gateway 的共享密钥，生产环境必须覆盖默认值。

## 排障

- `invalid request`：JSON 非法、字段不符合接口要求，或包含未知字段。
- `invalid account or accountVerifyToken`：`account` trim 后为空，或 `accountVerifyToken` 为空。
- `invalid email or password`：email/password 登录时邮箱不存在、密码错误，或请求字段为空。
- `login credential config invalid`：login 当前运行配置文件缺失、格式错误、账号配置重复，或存在空 email/password。
- `cache not available`：login 未发现 cache，cache 不可用，或 cache RPC 返回不可用。
- `gateway not available`：login 未发现可用 gateway，或所有 gateway `availableLoad` 为 0/连接不可用。
- `cache account uid is empty`：cache 消费 accountVerifyToken 成功但返回 uid 为 0，属于 cache 数据异常。
- `connect ticket invalid`：gateway 校验失败，重点检查 `ticketSecret`、目标 gateway key、uid 和票据过期时间。

## 待改进项

- 为 `/api/login/session` 增加 accountVerifyToken 重放、gateway 不可用、ticket 过期/篡改等边界测试。
- 后续应将 `emailPasswordUsers.password` 从明文升级为哈希。
- HTTP 响应可补充 trace id，方便串联 login、cache、gateway、online 日志。
- 后续如果需要断线重连，应在 gateway 侧设计重连票据或 cache userSession 续期策略，不应复用一次性 accountVerifyToken。
