# Online 服务

Online 服务负责 Account actor, 业务逻辑入口和 gateway stream 下行. 启动时会先加载并校验 `custom.gameConfigDir` 指向的共享游戏配置, 再初始化 gRPC selector, etcd 和服务注册. 当前 cache userSession 的写入, 删除, 续期和顶号编排已经迁到 gateway. 部署, 端口, 容器启动和验证命令见 `deploy/online/README.md`.

## 能力边界

- 接收 gateway 的 `OnlineBindUser`，绑定 Account actor。
- 接收 gateway 的 `OnlineUnbindUser`，按 `gatewayKey + userSession` 清理 actor。
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包。
- 通过 gateway stream 下发业务响应。
- 处理用户业务数据，例如 `AccountRecordReq`、`AccountCreateReq`、`RobotPingReq`。
- 绑定用户时从 cache 读取并校验 `AccountRecord`。
- 更新 `AccountRecord` 时调用 cache `CacheSetAccountRecord`。
- 启动阶段加载 `character.yaml`, `enemy.group.yaml`, `exp.yaml`, `pet.skill.yaml` 和 `pet.yaml`, 校验 YAML 结构, 服务端消费字段, 枚举, 数值范围和跨表引用.

不再承担：

- 不查询或写入 `user:{uid}:session`。
- 不编排顶号。
- 不维护 cache userSession TTL。
- 不处理 `heartbeatSession` 轮换。
- 不维护 cache userSession 中的 `onlineKey`; 该字段由 gateway 写入, 仅用于排障定位。
- 不校验角色名称, 描述, 颜色, sprite, 客户端 PNG, `.tpsheet` 或 frame 资源是否存在; 客户端资源完整性由 sa.desktop 校验.

## 共享游戏配置

`custom.gameConfigDir` 指向共享游戏配置目录. 未配置时默认读取当前工作目录下的 `config`.

需要存在的文件：

```text
character.yaml
enemy.group.yaml
exp.yaml
pet.skill.yaml
pet.yaml
```

配置加载顺序和 sa.desktop 保持一致: 先分别执行单表 `load` 校验, 再执行跨表 `check`, 最后保留 `assemble` 生命周期. `character.yaml` 在 server 侧只消费 `id` 和 `isRole`; server 侧 `assemble` 不检查客户端资源帧, 只保证服务端需要的 YAML 数据和跨表引用有效.

Docker 镜像会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`。

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
2. 调用 cache `CacheGetAccountRecord` 读取用户档案。
3. 校验 `AccountRecord.uid/account` 与请求一致。
4. 按 uid 获取或创建 Account actor。
5. Account actor 绑定本地状态: `gatewayKey`、`userSession`、account、clientIP、accountRecord。
6. 写入 `GAccountMgr.accounts[uid]`。
7. 返回 gateway。

gateway 在调用 `OnlineBindUser` 前已经完成 connectTicket 验签、旧连接顶号和 cache userSession CAS；OnlineBindUser 内部读取并校验 AccountRecord。
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
2. 查找本地 Account actor。
3. 本地 Account actor 不存在时直接返回成功。
4. Account actor 校验请求中的 gatewayKey 和 `userSession` 必须匹配本地状态。
5. 匹配时删除 `GAccountMgr.accounts[uid]`，清空本地 gatewayKey/userSession 状态并停止 actor。
6. 不匹配时忽略该解绑请求，防止旧请求误停新 actor。

cache userSession 是否删除由 gateway 调用 `CacheEndUserSessionCAS` 决定。
gateway 调用 `OnlineUnbindUser` 的默认超时时间以 `proto/online.grpc.proto` 中的 `methodOpt.timeout` 为准。

## 业务数据流

```text
client TCP
  -> gateway User actor
  -> gateway OnlineStreamTunnel client
  -> online OnlineStreamTunnel server
  -> online Account actor
  -> gateway stream
  -> client TCP
```

当前已实现业务：

- `AccountRecordReq`：返回 online 本地缓存的 `AccountRecord`。
- `AccountCreateReq`：由客户端发起角色创建, 请求携带 `character_slot_index`; online 在指定角色槽位为空时初始化服务端权威账号档案数据, 设置 `account_record_create_timestamp_ms`, 按 `used_uuid` 生成默认角色 `uuid/nick/asset_id=1000011` 和默认宠物记录, 写入 `character_record_list[character_slot_index]`, 再调用 `CacheSetAccountRecord` 写回; `uid/account/account_create_timestamp_ms` 必须来自 `OnlineBindUser` 绑定阶段已校验的 cache 档案。
- `RobotPingReq`：返回 seq、clientTime、serverTime 和 payload。

## 一致性约定

- 同 uid 的 online 业务处理通过 Account actor 串行执行。
- online actor 只接受匹配 `gatewayKey + userSession` 的解绑请求。
- online 不写 cache userSession, 因此不能作为“是否允许上线”的权威。
- `AccountRecord` 是账号级档案聚合根, `uid/account` 下管理多个角色; `character_record_list` 的数组下标是角色槽位, 空槽使用 `uuid == 0` 的 `CharacterRecord` 占位, 每个账号最多可用角色槽位数量由 proto 常量 `AccountRecordLimit_MaxCharacterSlotCount` 定义, 完整角色业务 key 是 `uid + uuid`。
- `CharacterRecord.asset_id` 是角色资源 ID/角色 ID 的权威字段; `asset_id_record_map` 只保存 HP、属性、创建时间等数值记录。
- `AccountRecord` 由 online 登录绑定时从 cache 读取, online 业务更新时再写回 cache。cache 只创建账号壳数据, 角色、宠物和后续业务数据由 online 初始化和维护。
- 本轮不迁移旧 cache `AccountRecord`; 已存在但缺少 `CharacterRecord.asset_id` 的档案视为旧格式, 开发环境需要清理 cache 或重新创建账号。

## 排障

- `account record mismatch`：online 从 cache 读取的 `AccountRecord` 与 uid/account 不一致。
- `user not online` 不再作为解绑失败条件，本地 Account actor 不存在会返回成功。
- 业务包无响应：检查 gateway stream 是否注册、online 是否有对应 uid actor。
- `AccountRecord` 缺少角色记录：检查客户端是否完成 `AccountCreateReq`, 以及 online 写回 cache 是否成功。
- `CharacterRecord.asset_id` 为 0 或非法：这是旧 cache 档案或服务端初始化异常, 开发环境清理 cache 后重新创建账号。
- `DeadlineExceeded`：gateway 调用 online 超时，检查 gateway `onlineRPCTimeout`、online 日志和 actor 是否阻塞。
- `load game config failed`: `custom.gameConfigDir` 缺失, 目录下共享 YAML 不完整, 或 YAML 结构, 枚举, 数值范围, 跨表引用校验失败.

## 后续建议

- 补 `OnlineBindUser` actor 绑定测试和重复绑定测试。
- 补 `OnlineUnbindUser` 不存在成功、userSession 不匹配忽略、匹配停止 actor 测试。
- 将业务 handler 和登录 actor 状态拆分得更清晰，减少 online 主流程文件大小。
