# Online 服务

Online 服务负责 Account actor, 业务逻辑入口和 gateway stream 下行. 启动时会先加载并校验 `custom.gameConfigDir` 指向的共享游戏配置, 再初始化 gRPC selector, etcd 和服务注册. 当前 cache accountSession 的写入, 删除, 续期和顶号编排已经迁到 gateway. 部署, 端口, 容器启动和验证命令见 `deploy/online/README.md`.

## 能力边界

- 接收 gateway 的 `OnlineBindAccount`，绑定 Account actor。
- 接收 gateway 的 `OnlineUnbindAccount`，按 `gatewayKey + accountSession` 清理 actor。
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包。
- 通过 gateway stream 下发业务响应。
- 处理账号业务数据，例如 `AccountRecordReq`、`CharacterCreateReq`、`RobotPingReq`。
- 处理角色上线/下线和进入场景, 例如 `CharacterOnlineReq`, `CharacterOfflineReq`, `SceneEnterReq`; 在线集合和 active character 只存在 Account actor 内存, 上下线时间戳和当前场景写入角色档案。
- 绑定账号时从 cache 读取并校验 `AccountRecord`。
- 更新 `AccountRecord` 时调用 cache `CacheSetAccountRecord`。
- 启动阶段加载 `character.yaml`, `enemy.group.yaml`, `exp.yaml`, `pet.skill.yaml`, `pet.yaml` 和 `scene.yaml`, 校验 YAML 结构, 服务端消费字段, 枚举, 数值范围和跨表引用.

不再承担：

- 不查询或写入 `account:{aid}:session`。
- 不编排顶号。
- 不维护 cache accountSession TTL。
- 不处理 `heartbeatSession` 轮换。
- 不维护 cache accountSession 中的 `onlineKey`; 该字段由 gateway 写入, 仅用于排障定位。
- 不校验角色名称, 描述, 颜色, sprite, 客户端 PNG, `.tpsheet` 或 frame 资源是否存在; 客户端资源完整性由 sa.desktop 校验.
- 不校验宠物名称, 栖息地, 出生地, 描述, sprite, 客户端 PNG, `.tpsheet` 或 frame 资源是否存在; 客户端展示和资源完整性由 sa.desktop 校验.

## 共享游戏配置

`custom.gameConfigDir` 指向共享游戏配置目录. 未配置时默认读取当前工作目录下的 `config`.

需要存在的文件：

```text
character.yaml
enemy.group.yaml
exp.yaml
pet.skill.yaml
pet.yaml
scene.yaml
```

配置加载顺序和 sa.desktop 保持一致: 先分别执行单表 `load` 校验, 再执行跨表 `check`, 最后保留 `assemble` 生命周期. `character.yaml` 在 server 侧只消费 `id` 和 `isRole`; `scene.yaml` 在 server 侧校验场景 ID 处于协议场景资源段, `enemyGroups` 非空且权重大于 0, 并在 `check` 阶段校验敌人组引用必须存在于 `enemy.group.yaml`; `pet.yaml` 在 server 侧只消费 `id`, `rarity`, `elemental`, `attribute`, `growth`, `skill`, 并在 `check` 阶段校验非 0 技能槽位必须存在于 `pet.skill.yaml`. server 侧 `assemble` 不检查客户端资源帧, 只保证服务端需要的 YAML 数据和跨表引用有效.

Docker 镜像会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`。

## OnlineBindAccount

请求字段：

```text
aid
account
gatewayKey
clientIp
accountSession
```

处理顺序：

1. 校验 aid、account、gatewayKey 和 `accountSession` 非空。
2. 调用 cache `CacheGetAccountRecord` 读取账号档案。
3. 校验 `AccountRecord.aid/account` 与请求一致。
4. 按 aid 获取或创建 Account actor。
5. Account actor 绑定本地状态: `gatewayKey`、`accountSession`、account、clientIP、accountRecord。
6. 写入 `GAccountMgr.accounts[aid]`。
7. 返回 gateway。

gateway 在调用 `OnlineBindAccount` 前已经完成 connectTicket 验签、旧连接顶号和 cache accountSession CAS；OnlineBindAccount 内部读取并校验 AccountRecord。
因此 online 不判断账号是否允许上线，也不创建抢占失败请求的 actor；只有已经抢到 cache accountSession 的 gateway 请求会进入 `OnlineBindAccount`。
gateway 调用 `OnlineBindAccount` 的默认超时时间以 `proto/online.grpc.proto` 中的 `methodOpt.timeout` 为准。

## OnlineUnbindAccount

请求字段：

```text
aid
gatewayKey
accountSession
reason
msg
```

处理顺序：

1. 校验 aid、gatewayKey 和 `accountSession` 非空。
2. 查找本地 Account actor。
3. 本地 Account actor 不存在时直接返回成功。
4. Account actor 校验请求中的 gatewayKey 和 `accountSession` 必须匹配本地状态。
5. 匹配时删除 `GAccountMgr.accounts[aid]`，清空本地 gatewayKey/accountSession 状态并停止 actor。
6. 不匹配时忽略该解绑请求，防止旧请求误停新 actor。

cache accountSession 是否删除由 gateway 调用 `CacheEndAccountSessionCAS` 决定。
gateway 调用 `OnlineUnbindAccount` 的默认超时时间以 `proto/online.grpc.proto` 中的 `methodOpt.timeout` 为准。

## 业务数据流

```text
client TCP
  -> gateway Account actor
  -> gateway OnlineStreamTunnel client
  -> online OnlineStreamTunnel server
  -> online Account actor
  -> gateway stream
  -> client TCP
```

当前已实现业务：

- `AccountRecordReq`：返回 online 本地缓存的 `AccountRecord`。
- `CharacterCreateReq`：由客户端发起角色创建, 请求携带 `character_slot_index`, `character_id`, `character_nick` 以及必填 `character_elemental/character_attribute`; `character_elemental` 包含地水火风四项, 每项必须在 0-10, 总和必须等于 10, 且只能是单元素或两个相邻元素; `character_attribute` 包含体力/腕力/耐力/速度四项, 每项必须在 0-20, 总和必须等于 20; 未提交或非法时返回 `InvalidArgument`; online 在指定角色槽位为空时初始化服务端权威账号档案数据, 设置 `account_record_create_timestamp_ms`, 按 `used_uuid` 生成指定或默认角色 `uuid/nick/asset_id`, 写入 `CharacterRecord.exp=0`, `elemental`, `available_point=0`, `attribute`, `scene_id=2000001`, `create_timestamp_ms` 和 `rebirth_count=0`, 不在 `asset_id_record_map` 写入方向和动作, 5 只默认宠物按 4000101/4000102/4000103/4000104/4000105 顺序追加到角色 `pet_record_list`, 5 只默认宠物 `PetRecord.exp=0`, `create_timestamp_ms`, `rebirth_count=0`, 品阶均为 `PetGrade_Mythic`, 第一只状态为 `Battle`, 其余状态为 `Wait`, 默认不写入账号 `pet_warehouse_record_map`, 再调用 `CacheSetAccountRecord` 写回; `aid/account/account_create_timestamp_ms` 必须来自 `OnlineBindAccount` 绑定阶段已校验的 cache 档案。
- `CharacterOnlineReq`：由客户端选择角色后发起, 请求携带 `character_uuid`; Account actor 校验角色属于当前账号, 未上线, 且角色 `scene_id` 存在于 `scene.yaml` 后, 写入角色最后登录时间戳并写回 cache, 再将 UUID 加入内存在线集合并设为 active character, 同时清理自动遇敌状态, 返回空 `CharacterOnlineRes`。
- `CharacterOfflineReq`：由客户端选择角色下线时发起, 请求携带 `character_uuid`; Account actor 校验角色属于当前账号且已上线后, 写入角色最后登出时间戳并写回 cache, 再从内存在线集合移除 UUID; 如果该角色是 active character, 同步清空 active character, 返回空 `CharacterOfflineRes`。
- `SceneEnterReq`：由客户端请求已上线角色进入指定场景, 请求携带 `character_uuid` 和 `scene_id`; Account actor 校验角色属于当前账号且已上线, `scene_id` 存在于 `scene.yaml`, 且当前不在战斗中, 然后写入角色 `scene_id`, 写回 cache, 设为 active character, 清理自动遇敌状态, 返回 `SceneEnterRes`。
- `RobotPingReq`：返回 seq、clientTime、serverTime 和 payload。
- `AutoEncounterSetReq`：设置 Account actor 运行期自动遇敌开关, 开启时要求存在 active online character, 当前场景有效且角色有 `Battle` 宠物; 开启后在非战斗状态每 5 秒检查一次并由 online 权威触发遇敌; 关闭, 切换场景, unbind 或 stop 会清理自动遇敌和回合 timer。
- `CombatBattleStartNotify`: 由 online 主动推送, `session_id=0`; 只包含 `CombatBattleStart` 开战快照。当前根据 active character 的 `scene_id` 从 `scene.yaml` 的 `enemyGroups` 按权重选择敌人组, 使用 active character 和其 `Battle` 宠物作为发起方, 敌方为被触发方。
- `CombatRoundPrepareNotify`: 开战后和每个未结束回合开始时推送, 包含当前回合号, server 时间, 100 秒 deadline, 全场 `CombatUnitState`, 可控单位 `CombatActionOption`, 必须提交和已提交单位列表; 对应旧版 `BP/BC/BA` 的 typed pb 表达。
- `CombatRoundReadyNotify`: 玩家单位每次合法提交后推送完整 ready 状态, 客户端不需要本地猜测已提交数量。
- `CombatRoundActionReq`: 单位级出手请求, 每条消息只提交一个 `CombatUnitKey + action_type + action_id + target_key`; 角色首版只接受攻击/防御, 宠物技能必须来自当前战斗宠物 `pet.yaml` 非 0 技能槽。同一单位同回合提交后锁定, 重复提交返回失败。
- `CombatRoundResultNotify`: 由 online 主动推送已结算回合, 只包含 `intent_list`, `event_list`, `battle_finished` 和 `settlement`; 下一回合入口统一由 `CombatRoundPrepareNotify` 表达。首版实现 HP delta, 防御减伤, 多段技能, 死亡, 胜负和空奖励结算; 阵营存在角色单位时, 该阵营全部角色死亡即判负, 该阵营没有角色单位时才按全阵营单位死亡判负。

## 一致性约定

- 同 aid 的 online 业务处理通过 Account actor 串行执行。
- online actor 只接受匹配 `gatewayKey + accountSession` 的解绑请求。
- online 不写 cache accountSession, 因此不能作为“是否允许上线”的权威。
- `OnlineBindAccount` 建立账号 session 时角色在线集合为空, 客户端必须显式发送 `CharacterOnlineReq` 后角色才允许进入在线状态。
- 角色在线集合和 active character 只存在 Account actor 内存中, 不写入 `AccountRecord`, 不写入 Redis/cache; `OnlineUnbindAccount` 清理 Account actor 时会给仍在线角色写入最后登出时间戳并清空这些运行期状态。
- `CharacterCreateReq` 只创建角色档案, 不会自动将角色加入在线集合。
- `AccountRecord` 是账号级档案聚合根, `aid/account` 下管理多个角色; `character_record_list` 的数组下标是角色槽位, 空槽使用 `uuid == 0` 的 `CharacterRecord` 占位, 每个账号最多可用角色槽位数量由 proto 常量 `AccountRecordLimit_MaxCharacterSlotCount` 定义, 完整角色业务 key 是 `aid + uuid`。
- `CharacterRecord.asset_id` 是角色资源 ID/角色 ID 的权威字段; `CharacterRecord.exp/elemental/available_point/attribute/scene_id/create_timestamp_ms/rebirth_count/last_login_timestamp_ms/last_logout_timestamp_ms` 直接保存角色经验、元素、可用点、基础状态、当前场景、创建时间、转生次数和上下线时间; `asset_id_record_map` 当前不承载角色资源 ID、经验、元素、属性、场景、创建时间、转生次数、上下线时间戳、方向和动作; 角色 HP 由基础状态计算, 不写入角色记录。
- `CharacterRecord.pet_record_list` 只保存角色当前随身携带宠物, 按携带顺序排列, 单角色最多携带 `PetRecordLimit_MaxCarryCount` 只; `AccountRecord.pet_warehouse_record_map` 是账号宠物仓库, 同账号下所有角色共享, 最多存放 `AccountRecordLimit_MaxPetWarehouseCount` 只.
- `PetRecord.exp/loyalty/saved_base_*/raw_*/create_timestamp_ms/rebirth_count` 直接保存宠物经验、忠诚度、成长基础值、当前原始属性、创建时间和转生次数; `asset_record_base_map` 只保存宠物资源 ID。`PetRecord.grade` 表示宠物品阶, 创建宠物时决定 SavedBase 成长偏移: `Common=-2`, `Rare=-1`, `Epic=0`, `Legendary=1`, `Mythic=2`; `Raw` 四维仍由 SavedBase 和现有初始/升级公式推导, 不额外叠加品阶倍率.
- `PetRecord.carry_status` 表示宠物携带状态; 角色随身宠物中最多一只 `Battle`, 最多一只 `Mount`, 仓库内宠物固定为 `Rest`. `Mount` 需要忠诚度 100 和骑乘权限, 当前仓库暂未建模骑乘权限字段.
- `AccountRecord` 由 online 登录绑定时从 cache 读取, online 业务更新时再写回 cache。cache 只创建账号壳数据, 角色、宠物和后续业务数据由 online 初始化和维护。
- 战斗状态首版只保存在 Account actor 运行期 `AccountCombatState`, 不写回 `AccountRecord` cache, 也不做掉线恢复.
- 回合流程由 Account actor 串行: `Start -> Prepare -> ActionReq/Ready -> Result -> Prepare`; 玩家角色和战斗宠各提交一条单位级动作后立即结算; 超过 100 秒未提交的角色默认防御, 战斗宠从自身非 0 技能槽随机选招, 敌方每回合从对应宠物模板非 0 技能槽随机选招.
- `CombatRoundResultNotify.intent_list` 只表示本回合选择或默认填充的出手意图, `CombatIntentSource` 区分玩家, 超时默认和敌方 AI; `event_list` 使用 `event_id + parent_event_id` 表达攻击, 反击, 合击和状态触发链; `CombatEvent.effect_list` 是可顺序回放的 typed 效果列表, 首版实际生成 `Damage`, `Guard`, `UnitAlive` 和必要的 `Miss`.
- HP, alive 和防御状态只存在 `AccountCombatState` 运行期, 不写回 `AccountRecord` cache; 战斗胜负优先看阵营角色单位是否全部死亡, 无角色单位的阵营退回看全阵营单位是否全灭; 战斗结束后若自动遇敌仍开启, 清理本场状态并重新注册 5 秒遇敌 timer, 下次仍按 active character 当前场景重新选择敌人组.
- 旧版战斗 token 映射: `BP` 对应 `CombatRoundPrepareNotify` 中的可控单位和时间窗口, `BC` 对应 `CombatBattleStart.unit_list + CombatRoundPrepareNotify.unit_state_list`, `BA` 对应 `CombatRoundReadyNotify`, `B_RECV` 对应 `CombatRoundActionReq`, 旧动作脚本 `BB/BD/BV/BE` 等表现节点对应 `CombatRoundResultNotify.event_list.effect_list`.
- 本轮不兼容旧 cache `AccountRecord`; 已存在但缺少 `CharacterRecord.asset_id/attribute` 等新结构化字段的档案视为旧格式, 开发环境需要清理 cache 或重新创建账号。

## 排障

- `account record mismatch`：online 从 cache 读取的 `AccountRecord` 与 aid/account 不一致。
- `account not online` 不再作为解绑失败条件，本地 Account actor 不存在会返回成功。
- 业务包无响应：检查 gateway stream 是否注册、online 是否有对应 aid actor。
- `AccountRecord` 缺少角色记录：检查客户端是否完成 `CharacterCreateReq`, 以及 online 写回 cache 是否成功。
- `CharacterRecord.asset_id` 为 0 或 `attribute` 缺失：这是旧 cache 档案或服务端初始化异常, 开发环境清理 cache 后重新创建账号。
- `auto encounter failed`: 检查账号是否已有 active online character 和 `Battle` 宠物, 角色 `scene_id` 是否存在于 `scene.yaml`, 场景是否配置可选敌人组, 以及 `pet.yaml`/`exp.yaml` 是否能生成宠物战斗属性.
- `invalid combat action`: 检查客户端提交的 `battle_id/round/unit_key` 是否匹配当前战斗, 角色动作是否只使用攻击/防御, 宠物技能是否来自当前战斗宠物模板技能槽.
- `DeadlineExceeded`：gateway 调用 online 超时，检查 gateway `onlineRPCTimeout`、online 日志和 actor 是否阻塞。
- `load game config failed`: `custom.gameConfigDir` 缺失, 目录下共享 YAML 不完整, 或 YAML 结构, 服务端消费字段, 枚举, 数值范围, 跨表引用校验失败. 角色和宠物展示字段或客户端资源完整性由 sa.desktop 校验, 不属于 online 启动失败原因.

## 后续建议

- 补 `OnlineBindAccount` actor 绑定测试和重复绑定测试。
- 补 `OnlineUnbindAccount` 不存在成功、accountSession 不匹配忽略、匹配停止 actor 测试。
- 将业务 handler 和登录 actor 状态拆分得更清晰，减少 online 主流程文件大小。
