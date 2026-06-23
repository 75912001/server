# Server 用户上下文

本文定义 server 项目中用户身份相关的统一语言, 用于让协议和服务文档围绕账号、用户和用户档案保持一致。

## 语言

**account**:
系统中的登录身份字符串, 与一个可信 `uid` 一一对应。它不是用户昵称, 也不是用户档案。
_Avoid_: username, name, nickname

**uid**:
用户的唯一标识, 也是用户档案的主键。它标识一个 `User`, 不标识一次登录或一次连接。
_Avoid_: account id, session id, connection id

**User**:
由 `uid` 标识的项目用户。它不同于 `account`, `account` 只是该用户的登录身份。
_Avoid_: account, connection, session

**UserRecord**:
某个 `uid` 对应的用户档案快照。`user_create_timestamp_ms = 0` 表示账号已存在, 但用户资料尚未完成创建。
_Avoid_: session, cache entry, profile fragment

**UserRecord extension record**:
`UserRecord` 中用于承载业务扩展记录的通用容器。它只描述记录的分组和存储形态, 不定义具体业务含义; 具体含义必须由业务协议或业务文档定义。
_Avoid_: business event, domain record, audit log

**connectTicket**:
login 签发给客户端的一次性连接票据, 绑定目标 gateway 身份、`uid` 和 `account`, 只用于首次 TCP 登录验证。同一票据重放必须失败, 且不能进入顶号或 online bind 流程; `gatewayAddr` 不进入 `connectTicket`。
_Avoid_: session, heartbeat token, accountVerifyToken

**accountVerifyToken**:
账号级一次性验证凭证, 由外部可信程序写入, 由客户端提交给 login 消费。消费成功后 cache 解析或创建可信 `uid`, login 再签发 `connectTicket`; 它不是 gateway 连接票据。
_Avoid_: connectTicket, heartbeatSession, session token

**gatewayAddr**:
客户端用于建立 TCP 连接的 gateway 网络地址。它不是 gateway 身份, 也不是 `connectTicket` 的组成部分。
_Avoid_: gatewayKey, ticket field, session identity

**gatewayKey**:
gateway 实例身份, 用于把 `connectTicket` 和服务间控制请求绑定到指定 gateway。它不是客户端连接地址。
_Avoid_: gatewayAddr, host, socket address

**onlineKey**:
online 实例身份, 用于描述某次用户连接当前绑定的 online。它只是 cache userSession 的定位元数据, 不决定用户是否允许上线, 也不参与连接身份判断。
_Avoid_: userSession, gatewayKey, online address

**userSession**:
一次已验证用户连接的稳定身份, 登录成功时生成, 在该连接生命周期内不轮换。它标识连接身份, 不标识用户本身, 也不标识心跳凭证。
_Avoid_: uid, heartbeatSession, connectTicket

**heartbeatSession**:
客户端与 gateway 之间的心跳认证凭证, 登录成功后下发并可随心跳轮换。它不标识用户连接身份, 也不由 login 签发。
_Avoid_: userSession, connectTicket, accountVerifyToken

**controlPlaneCaller**:
被明确授权调用 `serviceControlPlane` 的内部调用方. 它不同于客户端、公网调用方或用户身份, 也不表示所有内部服务默认拥有全部控制面权限.
_Avoid_: client, user, public caller, trusted service

**serviceControlPlane**:
login/gateway/online/cache 等服务之间用于控制连接、会话、绑定和踢下线等内部状态的调用边界. 它不属于客户端协议面, 不应直接暴露给客户端或公网调用方.
_Avoid_: client API, public API, user protocol
