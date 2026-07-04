# grpc/template

## Generate protobuf code (Including standard stubs & Go-gRPC-X extensions)

`./proto` 是服务端和 sa.desktop 共享协议的唯一源头。客户端仓库只在生成前从这里单向同步 `.proto` 文件, 不反向维护独立协议定义。

账号档案统一使用 `AccountRecord` 作为聚合根: `aid/account` 管理多个 `CharacterRecord`, `character_record_list` 的数组下标是角色槽位, 空槽使用 `uuid == 0` 的 `CharacterRecord` 占位, 每个账号最多可用角色槽位数量由 proto 常量 `AccountRecordLimit_MaxCharacterSlotCount` 定义, 完整角色业务 key 是 `AccountRecord.aid + CharacterRecord.uuid`。账号不保存昵称, 角色昵称保存在 `CharacterRecord.nick`; `CharacterRecord.asset_id` 是角色资源 ID/角色 ID 的权威字段, `CharacterRecord.exp/elemental/available_point/attribute/scene_id/create_timestamp_ms/rebirth_count/last_login_timestamp_ms/last_logout_timestamp_ms` 直接保存角色经验、元素、可用点、基础状态、当前场景、创建时间、转生次数和上下线时间, `asset_id_record_map` 当前不承载角色资源 ID、经验、元素、属性、场景、创建时间、转生次数、上下线时间戳、方向和动作, HP 由基础状态计算。角色 `pet_record_list` 只保存当前随身携带宠物, 按携带顺序排列, 上限由 `PetRecordLimit_MaxCarryCount` 定义; `PetRecord.exp/loyalty/saved_base_*/raw_*/create_timestamp_ms/rebirth_count` 直接保存宠物经验、忠诚度、成长基础值、当前原始属性、创建时间和转生次数, 宠物 `asset_record_base_map` 只保存宠物资源 ID; 账号 `pet_warehouse_record_map` 是同账号所有角色共享的宠物仓库, 上限由 `AccountRecordLimit_MaxPetWarehouseCount` 定义。宠物只能存在于某个角色随身携带列表或账号宠物仓库中的一个位置。协议不再维护重复的 `account.proto/AccountRecord` 或旧 `AccountRecord`; 本轮不兼容旧账号档案, 开发环境需要清理旧 cache 或重新创建账号。

To generate all files in `./proto/pb` (including `*.pb.go`, `*_grpc.pb.go`, and `*_grpc.x.pb.go`):

```bash
python gen.py
```

`gen.py` 会先根据源 proto 中的 `//0xHEX#...` 注释生成 `*.cmd.proto`。生成的 cmd 枚举值使用十进制数字, 注释保留原始十六进制命令号, 以同时兼容 `protoc` 和 sa.desktop 使用的 Godobuf 解析器。

```bash
go get github.com/75912001/xlib@latest
```

## 待办

- [ ] 补充服务器重启, 宕机, 账号重连正确性验证。覆盖 login accountVerifyToken/session, connectTicket, gateway AccountVerifyReq, cache accountSession CAS, online bind, heartbeat 恢复路径; 重点确认旧 cache accountSession 不阻塞重登, 顶号清理不误删新 cache accountSession, gateway/online 绑定最终一致。
- [ ] 修复 connectTicket 可重复使用问题。当前同一个 `connectTicket` 在有效期内可重复通过 gateway `AccountVerifyReq`, 并触发顶号; 应改为一次性消费语义, 首次验证成功后后续复用返回未认证, 且不进入顶号和 bind 流程。可选方案: 在 gateway 中缓存已使用 ticket/nonce 并定期清理过期数据, 或在 Redis 中记录票据已使用状态并通过原子操作消费。
- [ ] 收口 cache gRPC 控制面暴露风险。当前非服务调用方可直接调用 `CacheBeginAccountSessionCAS` 写入伪造 `account:{aid}:session`, 可能阻塞账号登录; 需要限制端口访问, 增加服务间鉴权, 并限制 cache accountSession TTL 上限。
- [ ] 收口 gateway gRPC 控制面暴露风险。当前 `GatewayKickAccountSession` 会断开账号连接并清理 cache accountSession, 但接口本身未做服务间鉴权; 需要限制端口只对可信内网服务开放, 并增加 mTLS 或 metadata token/HMAC 校验调用方身份。
- [ ] 修复 online `OnlineStreamTunnel` 未鉴权可绑定真实 `gateway_key` 的问题。当前非 gateway 调用方可能覆盖真实 gateway stream, 导致下行消息丢失或被劫持; 需要限制端口访问并校验 gateway 身份。
- [ ] 修复 online `OnlineStreamTunnel` 未鉴权可伪造任意在线 aid 上行业务帧的问题。当前非 gateway 调用方可绕过 gateway 登录态和 heartbeat 校验, 直接驱动 online 账号命令; 需要校验 stream 调用方身份和 gateway_key, 并确保账号帧绑定到可信 gateway accountSession。
- [ ] 收口 login `/api/login/accountVerifyToken` 写入入口。当前接口未鉴权可为任意 account 写入 accountVerifyToken, 进而通过 `/api/login/session` 获取 connectTicket; 需要将 accountVerifyToken 写入接口收口到可信认证服务, 增加鉴权或签名校验, 并避免直接暴露给客户端或公网。
