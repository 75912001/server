# Online 服务

Online 服务负责 Account actor、账号业务、角色业务和 PVE 战斗. gateway 负责连接、登录会话、顶号和断线编排.

部署、端口、容器启动和验证命令见 `deploy/online/README.md`.

## 能力边界

Online 当前负责:

- `OnlineBindAccount` 和 `OnlineUnbindAccount`.
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包并返回业务响应.
- 读取、校验和持久化 `AccountRecord`.
- 角色创建、上线、下线、场景切换和自动遇敌.
- 宠物携带状态、仓库存取和昵称.
- 道具使用、背包和仓库存取、全局武器商店购买, 以及角色武器装备、卸下和替换.
- 在线角色邮箱的获取、已读、删除和新增邮件通知.
- GM 结构化命令, 当前支持增加道具、宠物和系统邮件.
- 每场独立 `CombatRoom` actor 的 PVE 回合、动作和结算.

Online 不负责:

- 写入或续期 cache account session.
- 顶号和连接断开.
- 客户端图片、sprite、`.tpsheet` 或动画帧完整性校验.

业务失败只返回对应业务错误码. Online 返回的 `InvalidArgument`、`FailedPrecondition`、`NotFound` 或 `Internal` 不表示要求 gateway 断开用户连接. gateway 只有在连接协议、会话安全或服务自身明确进入断线流程时才断开.

Online 运行配置将 `grpc.maxReceiveMessageBytes` 和 `grpc.maxSendMessageBytes` 都设为 `67108864`, 即服务端和生成客户端的单条消息收发上限均为 64MiB.

## 共享游戏配置

`custom.gameConfigDir` 指向共享配置目录, 未配置时默认读取当前工作目录下的 `config`.

Online 启动必需文件:

```text
character.yaml
skill.yaml
enemy.group.yaml
enemy.exp.yaml
exp.yaml
item.yaml
pet.yaml
scene.yaml
```

Docker 镜像会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`.

配置加载依次执行单表 `load`、跨表 `check` 和 `assemble`. 文件缺失、字段非法或引用无效时直接启动失败, 不继续注册 etcd 或启动业务服务.

自动遇敌按 `scene.yaml -> enemy.group.yaml -> pet.yaml` 生成单位. 场景按权重选择敌人组, 敌人组的 `enemies[].id` 直接引用宠物模板; 数量和等级来自敌人组, 属性和 AI 来自宠物模板, 经验由 `enemy.exp.yaml` 结合宠物模板计算.

## 角色道具与独立资产

角色道具统一通过 `characterItemManager.Count/Add/Consume` 访问. `[3000000,3489999]` 的普通可堆叠道具存入 `item_bag.item_count_map` 并占用背包种类容量; `[3490000,3499999]` 的角色独立资产存入 `CharacterRecord.asset_count_map`, 不占背包容量且禁止存入账号共享仓库. 两类数据都必须存在于 `item.yaml`, 数量为 0 时删除映射键, 查询缺失键返回 0. 角色独立资产的合法数量上限为 `math.MaxInt64`, 避免超出 Godot 有符号 64 位整数范围.

协议 `AssetID` 枚举当前定义 `AssetID_Stone`、`AssetID_Silver` 和 `AssetID_Gold`. `CharacterItemChangedNotify` 对两类道具都携带最终数量, 调用方按 ID 范围选择目标容器. cache 档案中的角色资产若范围非法、数量超限、缺少配置或误存入角色背包/账号仓库, 账号绑定直接失败; 项目不迁移旧 `stone/shell/diamond` 字段.

## 全局武器商店

`ShopPurchaseReq` 购买 `item.yaml` 中 `cost > 0` 的武器, `cost` 当前固定使用 `AssetID_Stone` 计价. 商店没有 NPC、场景、库存或限购条件; 服务端仍会校验角色属于当前账号、已经在线且不在战斗中, 并校验客户端提交的预期单价、数量、石币资产、背包剩余格数和账号 UUID 游标. `cost = 0`、非武器 ID、价格不一致或数量非法都必须拒绝.

购买时先在账号档案副本中通过统一道具接口扣除石币资产, 为每件武器创建 `EquipmentRecord`. 每个实例保存 `uuid`、`asset_id`, 并把攻击、防御、敏捷、HP、MP、运气、魅力、闪避、六种状态抗性和暴击共15项配置闭区间分别随机一次, 完整固化到 `record_base_map`; 即使实际值为0也保留对应 key. 随后调用 cache 持久化完整 `AccountRecord`. cache 成功后才提交 Online 内存档案; 任一校验或持久化失败都不修改资产、背包和 `used_uuid`. `ShopPurchaseRes.affected_item` 返回 `AssetID_Stone` 及扣除后的最终持有数量, 同时返回成交单价、总价、最新 UUID 游标和本次新增装备列表, 供客户端原子替换权威快照.

## 角色武器换装

`CharacterEquipmentReplaceReq` 和 `CharacterEquipmentReplaceRes` 的消息 ID 分别为 `0x00100F` 和 `0x001010`. 请求携带角色 UUID、目标部位和背包装备 UUID; 当前只接受 `EquipmentType_Weapon`, `equipment_uuid=0` 表示卸下当前武器. 角色必须属于当前账号、已经在线且不在战斗中. 装备时还校验实例确实位于该角色背包、武器配置及类型有效、角色等级满足要求; 当前角色档案没有职业字段, 因此带 `neprof` 限制的武器明确拒绝.

换装在 Account actor 内串行构造完整账号候选档案. 装备或替换时从背包移出新武器, 并把旧武器放回背包; 卸下时要求背包仍有空位. cache 持久化成功后才同时提交角色背包、穿戴记录和运行中角色引用, 失败时权威档案不发生任何变化. 成功响应返回完整 `item_bag`、完整 `equipment` 和服务器重算后的 `effective_attribute`, 客户端按一次响应原子替换, 不在请求前预改本地档案.

账号绑定、角色上线和角色基础数据变化都会下发服务器派生的有效属性. 有效属性以角色基础值和当前装备实例的固化值计算, 并包含最大HP、攻防敏、最大MP、有效运气/魅力/闪避、暴击、六种状态抗性、元素、额外伤害/防御以及实际武器类型. 该快照不持久化; 每次换装或基础属性变化后重新计算. 当前不兼容旧装备实例: `record_base_map` 缺项、多项、越界或包含额外 key 时, 账号绑定、仓库存取或业务使用直接失败.

## 宠物创建与品阶

创建宠物时显式传入 `Common` 到 `Mythic`, 会分别对四项 SavedBase 统一应用 `-2`、`-1`、`0`、`1`、`2` 偏移. 传入 `Unknow` 表示随机品阶: 四项分别独立等概率随机整数偏移 `[-2,2]`, 再按总偏移计算并保存实际品阶, `PetRecord.grade` 不会保存 `Unknow`.

随机品阶按总偏移划分为 `Common=-8~0`、`Rare=1~2`、`Epic=3~4`、`Legendary=5~6`、`Mythic=7~8`, 对应概率为 `56.80%`、`23.68%`、`13.92%`、`4.80%`、`0.80%`.

## 统一技能模型

`skill.yaml` 是角色和宠物共用的唯一技能定义来源. `pet.yaml skill` 只保存宠物实际拥有的技能 ID, 不保存技能行为.

当前角色动作:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8000003`: 逃跑.
- `8000004-8000007`: 配置保留, 当前未开放. 请求返回 `InvalidArgument`, 不执行动作.

当前宠物解析器支持:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8100000`: 待机.
- `8100003`: 破除防御.
- 配置了 `continuationAttack.segmentCount` 的连续攻击.

宠物使用技能必须同时满足:

1. 技能存在于 `skill.yaml`.
2. 技能 ID 出现在该宠物模板的 `pet.yaml skill` 槽位.
3. Online 已实现对应行为.
4. 请求目标和阵营参数合法.

当前 `pet.yaml` 中所有宠物都只配置 `[8000001,8000002,0,0,0,0,0]`, 因此生产宠物当前只能主动使用攻击和防御.

未知技能、未开放技能、未持有技能和非法目标都返回 `CombatRoundActionRes` 的 `InvalidArgument`. 该响应不会触发 gateway 断线.

## 角色交互设置

`CharacterSettingSetReq` 修改当前账号登录会话内指定角色的组队和决斗开关. 状态保存在 Account actor 的角色管理器中, 不进入 `CharacterRecord`, 也不调用 cache 持久化.

每次 `OnlineBindAccount` 创建新的角色管理器时, 所有角色的组队和决斗开关都初始化为关闭. 客户端发送修改请求后保持原显示状态, 仅在收到 `CharacterSettingSetRes` 成功回复后应用服务端返回的权威值; 请求失败时继续显示原状态.

## 角色属性加点与重置

`CharacterAttributeAddReq` 每次只为当前账号的一名在线且非战斗角色增加 1 点基础属性. 可选属性严格限定为体力、腕力、耐力和速度, 魅力不在本协议范围内. 服务端验证角色归属、在线状态、战斗状态、可加点数和 `uint32` 溢出后, 克隆角色档案并减少 1 点 `available_point`; cache 持久化失败时保留原档案.

持久化成功后先发送 `CharacterBaseChangedNotify` 权威快照, 再回复 `CharacterAttributeAddRes`. 客户端不得乐观修改属性, 成功响应仅用于解除请求等待状态.

`CharacterAttributeResetReq` 提交体力、腕力、耐力、速度的最终值和剩余 `available_point`. 服务端只接受当前账号中已上线且不在战斗中的角色, 当前权威总点数必须在 20 至 1000 之间, 最终四项至少分配 20 点, 并且最终四项与剩余点数之和必须严格等于重置前的权威总点数.

重置时先构造独立的候选账号档案并交给 cache 持久化, cache 返回失败则此次操作失败且 online 权威档案保持不变; cache 成功后才提交 online 角色档案, 随后发送 `CharacterBaseChangedNotify` 和 `CharacterAttributeResetRes`. 装备属性要求等后续限制可在持久化前增加校验并返回明确错误.

## 角色邮箱

角色邮箱由 Cache 的独立 Redis hash 持久化, 不进入 Online 内存中的 `AccountRecord`. Online 只为当前 Account actor 内已经上线且属于该账号的角色同步转发以下请求:

```text
CharacterMailboxGetReq    -> CacheGetCharacterMailbox    -> CharacterMailboxGetRes
CharacterMailReadReq      -> CacheMarkCharacterMailRead  -> CharacterMailReadRes
CharacterMailDeleteReq    -> CacheDeleteCharacterMail    -> CharacterMailDeleteRes
```

客户端在角色上线成功后读取一次完整邮箱, 之后依靠 `CharacterSystemMailNotify` 合并新邮件. 邮件不存在或已经过期时, read/delete 返回 `NotFound`; Online 不把这类业务错误升级为断线。

当前系统邮件发送入口是在线角色自己执行的结构化 GM 命令 `gm mail add <主题>|<正文>`. Online 校验角色在线和文本后同步调用 `CacheAddSystemMail`; Cache 持久化成功后, Online 先发送包含完整 `MailRecord` 的 `CharacterSystemMailNotify`, 再发送 `GMCommandRes.mail_add`. 当前不限制系统邮件发送频率, 允许登录等业务连续补发多封; 不实现每日/每周/每年计数, 角色事件管理完成后再扩展. 当前不实现玩家互发、离线投递、附件、幂等键或发送记录。

## 战斗流程

```text
CombatBattleStartNotify
  -> CombatRoundActionReq
  -> CombatRoundActionRes
  -> CombatRoundResultNotify
  -> CombatFlowCompleteReq
```

- `CombatBattleStartNotify` 下发 `battle_id` 和初始单位列表.
- 角色和当前参战宠物分别提交动作.
- 每个单位每回合只能提交一次合法动作.
- 所有存活可控单位提交后立即结算; 超时单位使用默认防御.
- `CombatRoundResultNotify.event_list` 和其中的动作、效果数组顺序就是客户端播放顺序.
- 战斗结束后结算敌人经验, 再等待客户端完成表现流程.

基础物理战斗当前保留普通攻击、防御、逃跑、待机、破防、连续攻击、合击、反击、伤害、死亡和 PVE 结算. 开战快照复制角色完整装备和服务器有效属性. 玩家装备武器时, 每次实际行动都在命令分派前从配置 `attacknum_min/attacknum_max` 闭区间锁定一次段数; 非普通攻击只保留这次随机消耗, 不使用该段数. 爪普通攻击按本次计划总段数分摊每段正伤害, 其他当前武器每段保持完整普通攻击伤害. 武器实例固化的攻防敏、HP、MP、运气、暴击和配置元素、额外伤害/防御进入当前开战与物理结算; 武器类型还参与反击相性, 弓、回旋镖、投掷斧和投掷石禁止双方反击. 武器槽明确为空且等级达到10的玩家角色执行普通攻击时, 继续按BaseLuck锁定1、2、3或5至10段, 不会出现4段; 真正空手的每段都使用完整普通攻击伤害. 除弓和回旋镖专用目标展开外, 原目标死亡后每个剩余段都独立重新随机选择当时的存活敌方, 上一段的随机目标不会被锁定; 攻击者死亡、敌方全灭或段数耗尽时结束, 整组完成后仅按最后实际段判断一次反击. 合击和反击不展开为武器或空手多段. 合击每个成员都下发自己的`CombatEffect.damage`, 前序成员以`displayed_damage=0`和空`unit_delta_list`只表达命中、暴击及防守表现, 最后一名成员才携带合击总表现伤害和累计 HP 差量. 装备附带魔法当前仅作为展示元数据, 尚不执行施法; 投掷斧、投掷石的专用目标/消耗、状态抗性和其他特殊装备行为仍未完整接入.

### 回旋镖普通攻击

回旋镖按8.5服务端 `BATTLE_COM_BOOMERANG` 和 `BoomerangVsTbl` 扫描声明目标所在的一排. 声明目标只选择前排0至4或后排5至9; 即使声明目标在本动作执行前已经死亡, 仍保留其原排. 只有该排已经没有任何存活且未离场的敌方单位时, 才从全部存活敌方中随机选择一次回退目标, 并改扫回退目标所在排. 随机数继续使用当前每场战斗独立PCG, 不要求与旧服全局`rand()`算法或相同seed序列一致.

| 攻击方阵营 | 排内position偏移顺序 |
| --- | --- |
| Initiator, 原版side 0 | 4, 2, 0, 1, 3 |
| Defender, 原版side 1反向遍历 | 3, 1, 0, 2, 4 |

服务端按表中五个位置依次扫描, 空位、死亡和已离场单位只跳过; 每个有效位置完整执行一次独立物理攻击并产生一个有序`CombatActionStep`. 回旋镖不会使用配置`attacknum`限制实际命中目标数: 装备动作前仍按通用规则消费一次攻击次数随机, 但只要同行存在5个有效单位就会各攻击一次. 扫描途中击倒某个目标后继续后续位置, 不回头重复攻击, 也不把剩余次数集中补到单体.

每个目标先执行完整的闪避、暴击、Guard和普通伤害结算, 再按8.5的C `float`语义乘`gBattleDamageModyfy=0.3`并向零截断. 该缩放不补最低1点, 因而缩放前1至3点正伤害会得到0, 但仍保留原本的Normal或Critical命中表现; 缩放前已经是0伤害则保持Miss/Guard语义. 每个目标独立应用HP和死亡差量. 回旋镖属于投掷武器, 在合击资格随机前排除且攻守任一方装备回旋镖时不进入反击随机.

当前PVE尚无原版守护代挡、光镜守、陷阱、针刺及狐狸/猪变身的可达运行态. 后续接入这些系统时, 回旋镖必须继续按原版投掷分支跳过守护替换和普通反应降级链, 不能把当前不可达输入误写成已完成能力.

### 弓普通攻击

弓按8.5服务端 `BATTLE_TargetListSet` 的 `aBowW` 生成一次十站位候选表. 声明目标的阵营和站位只决定候选表, 不把全部箭锁在声明目标上. 当前协议每个阵营使用0至9站位, `column = targetPosition % 5`; 每场攻击只从当前战斗PCG抽取一次 `variant = RAND(0,1)` 等价分支, 随机算法本身不要求与旧服全局 `rand()` 逐位一致.

| column | variant 0同行顺序 | variant 1同行顺序 |
| --- | --- | --- |
| 0 | 0, 2, 1, 4, 3 | 0, 1, 2, 3, 4 |
| 1 | 1, 0, 3, 2, 4 | 1, 3, 0, 2, 4 |
| 2 | 2, 4, 0, 1, 3 | 2, 0, 4, 1, 3 |
| 3 | 3, 1, 0, 2, 4 | 3, 1, 0, 2, 4 |
| 4 | 4, 2, 0, 1, 3 | 4, 2, 0, 1, 3 |

同行顺序中的每个位置后立即插入另一排的同列位置, 最终十个站位各出现一次. 服务端从头扫描候选表: 空位、死亡或已离场单位只跳过, 不消耗 `attacknum`; 每次实际执行 `BATTLE_Attack` 等价结算才消耗一箭. 达到锁定的 `attacknum`、十站位扫描完毕、攻击者失效或任一阵营全灭时停止. 因为每个站位最多出现一次, 同一次弓动作不会再次攻击已经扫描过的位置; 某目标被箭击倒后会继续扫描后续站位, 不会把剩余箭补射到该目标.

每支箭独立执行闪避、暴击、Guard、最低伤害和HP变化, 并产生一个按结算顺序排列的 `CombatActionStep`. 弓暴击仍按普通暴击阈值判定并设置 `critical=true`, 但严格保留原版例外: 只使用普通 `BATTLE_DamageCalc` 等价伤害, 不追加非弓暴击的防御与等级增伤. 弓属于投掷武器分类, 在合击候选随机前即排除, 攻守任一方持弓也不会进入反击随机. 原版 `_FIXBUG_ATTACKBOW` 还会在道具/NPC/狐狸变身期间禁用远程武器; 当前PVE服务端尚无对应可达变身运行态, 后续接入该状态时必须在生成弓候选表前拒绝动作. 原版守护代挡运行态当前同样尚未接入; 接入时弓必须继续跳过守护者替换.

## 账号绑定与解绑

`OnlineBindAccount`:

1. 校验 aid、account、gatewayKey 和 accountSession.
2. 从 cache 读取并校验 `AccountRecord`.
3. 创建或绑定 Account actor.
4. 建立角色管理器并保存 gateway stream.

`OnlineUnbindAccount`:

1. 校验 gatewayKey 和 accountSession 与当前 actor 匹配.
2. 持久化仍在线角色的登出状态.
3. 取消自动遇敌和战斗引用.
4. 停止 Account actor.

cache account session 的结束由 gateway 负责.

## 排障

- `load game config failed`: 检查 `custom.gameConfigDir`、必需 YAML 文件和跨表引用.
- `invalid combat action`: 检查 battle ID、round、单位归属、技能是否存在、宠物是否持有技能及目标是否合法.
- `character mailbox is full`: 角色清理过期邮件后仍有 1000 封有效系统邮件, 新增邮件失败。
- 读取、已读或删除邮件返回 `NotFound`: 角色或邮件不存在, 或邮件已经过期; 客户端应移除本地对应邮件。
- online 返回业务错误后连接仍在: 这是预期行为, 客户端可修正请求后继续使用当前连接.
- 业务包无响应: 检查 gateway stream、online Account actor 和目标 aid.
- `DeadlineExceeded`: 检查 gateway 到 online 的 RPC 超时、online 日志和 actor 是否阻塞.

## 后续建议

- 为 `OnlineBindAccount` 和 `OnlineUnbindAccount` 补齐 actor 生命周期集成测试.
- 继续拆分较大的战斗结算文件, 但不要恢复已移除的旧技能兼容层.
