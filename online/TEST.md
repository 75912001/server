# Online 服务测试指南

## 适用范围

修改 online actor 绑定/解绑流程, Account actor 状态, gateway stream 路由, 共享游戏配置加载, `AccountRecordReq/CharacterCreateReq`, 自动遇敌, 回合出手业务 handler 或 online gRPC handler 时, 使用本文档.

## 快速检查

```bash
go test ./common/gameconfig ./online ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./online
```

## 依赖检查

当修改 gateway cache accountSession 编排、gateway 路由、login 或 proto 契约时，运行：

```bash
go test ./online ./gateway ./cache ./login ./tool/robot/main ./proto/pb
```

## 运行时依赖

手动验证 online 需要：

- `bin/online.yaml` 中的 etcd 地址
- `bin/online.yaml` 中的 online gRPC 监听地址
- `bin/online.yaml` 中的 `custom.gameConfigDir`, 本地 bin 启动默认应指向项目根目录 `config`
- `custom.gameConfigDir` 下存在 `character.yaml`、`enemy.group.yaml`、`exp.yaml`、`pet.skill.yaml`、`pet.yaml` 和 `scene.yaml`
- 已注册到 etcd 的 gateway 服务，用于下行路由
- 已注册到 etcd 的 cache 服务，用于账号档案和 cache accountSession 状态

## 手动验证

典型检查项：

- `OnlineBindAccount` 会从 cache 读取 account record；登录票据校验和 cache accountSession 编排由 gateway 负责
- online 启动会先加载共享游戏配置; 配置缺失, 服务端消费字段非法或跨表引用错误时应启动失败, 且不继续注册 etcd/gRPC
- `pet.yaml` 的 server 校验范围只包含 `id`, `rarity`, `elemental`, `attribute`, `growth`, `skill` 和技能引用; 宠物名称, 栖息地, 出生地, 描述, sprite, PNG, `.tpsheet` 和 frame 完整性由 sa.desktop 校验
- `CharacterCreateReq` 会按客户端传入的 `character_slot_index`, `character_id`, `character_nick` 以及必填 `character_elemental/character_attribute` 在 cache 账号壳档案上初始化 `account_record_create_timestamp_ms/used_uuid/character_record_list/pet_warehouse_record_map`, 写入 `CharacterRecord.exp=0`, `elemental`, `available_point=0`, `attribute`, `scene_id=2000001`, `create_timestamp_ms` 和 `rebirth_count=0`, 在 `asset_id_record_map` 写入方向 down 和动作 stand, 不写入角色 HP, 并写回 cache; 未提交元素或基础状态, 元素单项超出 0-10, 元素总和不等于 10, 两个非相邻元素组合, 基础状态单项超出 0-20 或基础状态总和不等于 20 都应返回非法参数
- 新账号首次登录后不会自动创建角色; 玩家显式发送 `CharacterCreateReq` 后能拿到默认角色和 5 只默认随身携带宠物, 默认账号宠物仓库为空; 重启或重登后通过 `AccountRecordReq` 能读回同一份 `AccountRecord`
- `CharacterCreateReq` 不会自动让角色上线; 客户端需要显式发送 `CharacterOnlineReq(character_uuid)` 才会在 Account actor 内存中标记该角色在线
- `CharacterOnlineReq` 对不存在角色, `character_uuid=0`, 已在线角色或记录中 `scene_id` 无效应返回错误; 成功时应写入角色 `last_login_timestamp_ms` 并写回 cache, 将该角色设为 active character, 响应为空 body
- `CharacterOfflineReq` 对不存在角色, `character_uuid=0` 或未在线角色应返回错误; 成功时应写入角色 `last_logout_timestamp_ms` 并写回 cache, 再从 Account actor 内存在线集合移除该角色 UUID; 如果该角色是 active character, 应清空 active character, 响应为空 body
- `SceneEnterReq` 对不存在角色, `character_uuid=0`, `scene_id=0`, 未上线角色, 不存在场景或战斗中切换场景应返回错误; 成功时应写入角色 `scene_id`, 写回 cache, 返回 `SceneEnterRes.character_uuid/scene_id/server_timestamp_ms`, 并取消自动遇敌
- 账号离线或 Account actor 停止时, 仍处于在线集合中的角色应写入同一个 `last_logout_timestamp_ms`, 并清空内存在线集合
- 新建 `AccountRecord.aid > 0`, `character_record_list` 非空, 至少一个角色 `uuid > 0`, 默认 `CharacterRecord.asset_id == 1000011`, 指定创建时应保存请求中的可创建角色 ID、昵称、`elemental` 和 `attribute`, 默认角色按顺序携带 4000101/4000102/4000103/4000104/4000105 五只宠物, 五只默认宠物 `PetRecord.exp=0`, 品阶均为 `PetGrade_Mythic`, 第一只为 `Battle`, 其余为 `Wait`, 默认账号 `pet_warehouse_record_map` 为空
- 新建默认宠物应写入 `loyalty`, `saved_base_*`, `raw_*`, `create_timestamp_ms` 和 `rebirth_count=0` 直字段; `asset_record_base_map` 只保存宠物资源 ID; PetGrade 到 SavedBase 偏移映射为 `Common=-2`, `Rare=-1`, `Epic=0`, `Legendary=1`, `Mythic=2`, 默认赠送宠物使用 Mythic 的 `+2` 偏移; 战斗单位属性从这些字段换算, 旧 cache 缺失新结构化字段时不会静默迁移
- `AutoEncounterSetReq(enabled=true)` 要求存在 active online character, 有效场景和 `Battle` 宠物; 成功后非战斗状态下 5 秒 timer 可触发 `CombatBattleStartNotify`; 战斗中不重复开战, `enabled=false`, 切换场景, unbind 和 stop 会清理 timer
- 自动遇敌应根据 active character 的 `scene_id` 从 `scene.yaml` 的 `enemyGroups` 按权重选择敌人组, 不再从全局 `enemy.group.yaml` 列表随机
- `CombatBattleStartNotify` 应只包含开战快照, 且包含玩家角色, 玩家 `Battle` 宠物和敌方单位; 敌人选择遵循普通组必出怪加权补齐, Boss 固定顺序, 等级优先 `enemies[].level`, 其次 `levelRange`, 其次 `roleLevelOffset`
- `CombatRoundPrepareNotify` 应在开战后和每个未结束回合开始时推送, 包含 round, server 时间, deadline, 全场 `CombatUnitState`, 可控单位动作选项, required/ready 单位列表
- `CombatRoundActionReq` 每次只提交一个出手单位; 合法角色攻击/防御和战斗宠技能应返回 `CombatRoundActionRes`, 随后推送 `CombatRoundReadyNotify`; 两个玩家单位动作都提交后应推送 `CombatRoundResultNotify`
- `CombatRoundActionReq` 的 battle/round 不匹配, 重复提交, 非玩家单位, 非法角色动作, 宠物技能不在当前战斗宠技能槽, 攻击/技能目标不是敌方存活单位时应拒绝
- 100 秒回合超时应默认角色防御, 战斗宠随机使用自身非 0 技能槽, 敌方随机使用对应宠物模板非 0 技能槽; `CombatRoundResultNotify.intent_list` 应记录本回合双方意图和 `CombatIntentSource`, `event_list.effect_list` 应按执行顺序生成 Damage/Guard/UnitAlive 等 typed 效果
- 伤害公式, 防御减伤, 多段技能, 死亡后跳过出手, 目标死亡后自动重选, 阵营角色全灭立即判负, 无角色阵营全灭立即结束, 空奖励结算和不写 cache 都需要有单测覆盖
- 校验角色随身携带宠物不超过 `PetRecordLimit_MaxCarryCount`, 账号宠物仓库不超过 `AccountRecordLimit_MaxPetWarehouseCount`, 同一宠物 UUID 不同时存在于角色携带列表和账号仓库
- online 只绑定 Account actor，不写入 cache accountSession 到 cache
- 重复登录会正确删除或替换旧 cache accountSession
- gateway stream 能接收下行 frame
- 解绑只在 `gatewayKey + accountSession` 匹配时清理 actor，不写 cache accountSession
