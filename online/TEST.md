# Online 服务测试指南

## 适用范围

修改 Account actor、共享配置加载、角色、宠物、商店或邮箱业务、`CombatRoom`、回合动作、gateway stream 路由或 online gRPC handler 时使用本文档.

## 快速检查

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./online
```

涉及 gateway、cache、login 或协议契约时追加:

```bash
GOCACHE="$PWD/.gocache" go test ./gateway ./cache ./login
```

## 配置检查

`custom.gameConfigDir` 必须包含:

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

验证要点:

- 缺少任一必需文件时, online 明确启动失败.
- `skill.yaml` 的 ID、名称、说明和连续攻击段数非法时加载失败.
- `item.yaml` 必须使用 `items.<group>.<id>` 结构; 未知或空分组、ID超出分组区间、非法 `atlas` 路径、武器分组混用图集、15项固化数值范围或攻击次数范围倒置、非法职业、非法武器类型或非法元素配置均应加载失败.
- 普通道具 `sprite` 为0时不能配置 `atlas`, `sprite` 大于0时必须配置以 `item/` 开头的无扩展名 `atlas`; 武器还必须使用非零 `sprite` 且不能配置 `use`.
- `pet.yaml skill` 的非 0 ID 不存在于 `skill.yaml` 时加载失败.
- `scene.yaml` 引用不存在的敌人组时加载失败.
- `enemy.group.yaml enemies[].id` 引用不存在的宠物模板时加载失败.
- 当前所有宠物技能槽都是 `[8000001,8000002,0,0,0,0,0]`.
- 配置加载失败后不注册 etcd, 不启动 gRPC 业务入口.

## 全局武器商店购买

- `ShopPurchaseReq` 和 `ShopPurchaseRes` 的消息 ID 分别保持为 `0x003007` 和 `0x003008`.
- 仅允许购买武器 ID 且 `item.yaml cost > 0` 的条目; `cost = 0`、非武器 ID、预期单价不一致和数量为0必须拒绝.
- 角色必须属于当前账号、已经在线且不在战斗中. `asset_count_map[AssetID_Stone]` 不足、背包满、数量超过剩余格数或 UUID 游标耗尽时不得修改账号档案.
- 成功购买多件武器时, 新装备 UUID 必须从旧 `used_uuid + 1` 连续递增. 每件记录除 `uuid` 和 `asset_id` 外, 必须完整保存15个 `EquipmentRecordBase` 固化 key; 每项值在对应配置闭区间内且不同实例分别创建, 并一次性扣除总价.
- cache 持久化失败必须保留原角色资产、背包和 UUID 游标; 成功响应的 `affected_item` 必须返回 `AssetID_Stone` 和扣除后的最终持有数量, 同时返回最新游标和完整的本次新增装备列表.

角色道具与独立资产还必须验证:

- 普通道具通过统一管理器写入背包并受容量限制, `[3490000,3499999]` 通过同一接口写入 `asset_count_map` 且不初始化背包.
- 角色资产扣减至 0 时删除键, 查询缺失键返回 0, 增加后不得超过 `math.MaxInt64`.
- 角色资产禁止在角色背包和账号仓库中出现, 仓库存取请求必须拒绝该范围.
- cache 档案中的角色资产 ID 越界、数量超限或缺少 `item.yaml` 配置时, `validateAccountRecord` 必须拒绝.

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./online -run ShopPurchase
```

## 角色武器换装

- `CharacterEquipmentReplaceReq` 和 `CharacterEquipmentReplaceRes` 的消息 ID 分别保持为 `0x00100F` 和 `0x001010`; 当前只接受 `EquipmentType_Weapon`, `equipment_uuid=0` 表示卸下.
- 只允许当前账号中已上线且不在战斗中的角色换装. 装备 UUID 必须位于该角色背包, 记录必须恰好含15个固化 key 且值位于当前配置闭区间; 缺项、多项、额外 key、UUID/配置不匹配均应拒绝.
- 装备时校验角色等级. 当前角色没有职业档案字段, 所有 `neprof != 0` 的武器都必须明确拒绝, 不得把未知职业当成满足要求.
- 替换时新武器进入武器槽、旧武器回到背包; 卸下时当前武器回到背包且必须有剩余容量. 计划构造和 cache 持久化失败不得修改原背包、穿戴记录或运行中角色引用.
- 成功响应必须同时返回完整 `item_bag`、完整 `equipment` 和同一候选档案计算出的 `effective_attribute`. 连续请求由 Account actor 按收到顺序串行处理, 不依赖客户端禁止重复操作.
- 账号校验必须覆盖背包、仓库和已穿戴武器之间的 UUID 全局唯一性, 并拒绝当前尚未开放的八个非武器穿戴部位.

## 统一技能测试

角色:

- `8000001` 生成普通攻击动作并校验敌方目标.
- `8000002` 生成以自身为目标的防御动作.
- `8000003` 生成逃跑动作.
- `8000004-8000007` 已配置但未开放, 必须返回业务错误且不写入本回合动作.
- `skill.yaml` 不存在的 ID 必须拒绝.

宠物:

- 技能必须同时存在于 `skill.yaml` 和该宠物的 `pet.yaml skill` 槽位.
- `8000001`、`8000002` 分别生成攻击和防御动作.
- 测试配置分配后, `8100000`、`8100003` 和连续攻击应由统一解析器生成对应动作.
- 未持有技能即使存在于 `skill.yaml` 也必须拒绝.
- 未实现行为即使已配置并持有也必须拒绝, 不允许回退成普通攻击、防御或待机.

错误响应:

- 非法动作返回 `CombatRoundActionRes` 对应错误码.
- gateway 不因 online 的普通业务错误码断开连接.
- 同一连接修正请求后仍可继续提交其他业务包.

## 角色交互设置

- `CharacterSettingSetReq` 和 `CharacterSettingSetRes` 的消息 ID 分别保持为 `0x001009` 和 `0x00100A`.
- 每次创建新的角色管理器时, 所有角色的组队和决斗状态都必须为关闭, 不继承上一次登录会话状态.
- 修改状态只更新 Account actor 当前会话内存, 不修改 `CharacterRecord`, 不调用 `CacheSetAccountRecord`.
- 角色 UUID 不属于当前账号或消息无法解析时必须返回业务错误, 不修改原状态.
- 客户端仅在成功回复后应用服务端返回值, 失败时保留请求前状态.

## 角色属性加点与重置

- `CharacterAttributeAddReq` 和 `CharacterAttributeAddRes` 的消息 ID 分别保持为 `0x00100B` 和 `0x00100C`.
- 体力、腕力、耐力和速度分别只增加目标字段 1 点并扣除 1 点 `available_point`; 魅力及未指定枚举必须拒绝.
- 非当前账号角色、未上线角色、战斗中角色、无可加点角色和目标字段 `uint32` 溢出必须拒绝且不得修改档案.
- cache 保存失败必须回滚账号槽位和角色内存档案; 保存成功后必须先发送 `CharacterBaseChangedNotify`, 再发送成功响应.
- `CharacterAttributeResetReq` 和 `CharacterAttributeResetRes` 的消息 ID 分别保持为 `0x00100D` 和 `0x00100E`.
- 重置只接受当前账号中已上线且不在战斗中的角色; 当前权威总点数必须在 20 至 1000 之间, 最终四项至少分配 20 点, 最终四项与剩余 `available_point` 之和必须严格守恒.
- 重置使用独立候选账号档案写 cache; cache 返回前不得修改 online 权威账号槽位或运行中角色档案, cache 失败时操作失败且原档案保持不变, cache 成功后才提交并发送权威变化通知和成功响应.
- 运行 `go test ./online -run 'CharacterAttribute(Add|Reset)|PrepareCharacterAttribute|PersistCharacterAttribute'` 覆盖消息 ID、字段映射、克隆隔离、边界校验、总点数守恒及持久化提交/失败不提交.

## 角色邮箱

- `CharacterMailboxGet/CharacterMailRead/CharacterMailDelete` 请求和响应 ID 保持为 `0x005000-0x005005`, `CharacterSystemMailNotify` 保持为 `0x005006`。
- get/read/delete 只接受当前账号已上线角色; 非法参数、非本账号角色和离线角色分别返回对应业务错误, 不导致 gateway 断线。
- `GMCommandReq.mail_add` oneof tag 保持为 12. Cache 持久化成功后必须先发送完整邮件通知, 再发送 GM 成功响应; Cache 失败时两者都不得发送成功结果。
- 主题最多 30 个 Unicode 字符, 正文最多 500 个 Unicode 字符; 正文 CRLF/CR 规范化为 LF, 非法控制字符必须拒绝。
- 运行 `go test ./common ./cache ./online -run 'Mail|Mailbox'` 覆盖文本规范化、邮箱 hash 解码、消息序列化和 GM oneof。

## 基础战斗回归

- 攻击、闪避、暴击、伤害、权威 HP 和死亡结果一致.
- 10级以上且装备快照武器槽为空的玩家普通攻击按BaseLuck生成1、2、3或5至10段, 永不生成4段; 每段保持完整普通攻击伤害, 原目标死亡后剩余段改选存活敌方, 攻击者死亡、敌方全灭或段数耗尽时停止, 整组只检查一次反击.
- 玩家已装备武器时, 每次实际行动必须在命令分派前只从 `attacknum_min/attacknum_max` 闭区间抽取一次段数; 非普通攻击仍消耗该随机值, 但不使用其段数或爪分摊. 爪普通攻击按计划总段数分摊正伤害, 原目标死亡后每个剩余段独立重选存活敌方; 弓以外的其他当前武器不分摊. 缺失装备快照不得误触发武器路径, 合击成员与反击仍保持单段.
- 开战快照使用服务器有效属性和实例固化的运气/暴击值, 元素及额外伤害/防御来自当前武器配置; 更换同配置但固化值不同的实例时, 战斗数值必须采用实际实例而不是重新随机.
- 回旋镖必须按声明目标所在排扫描. Initiator排内顺序为`4,2,0,1,3`, Defender为`3,1,0,2,4`; 后排在各偏移上加5. 声明目标死亡仍保留原排, 整排无有效目标时才从全部存活敌方随机一次决定回退排. 空位、死亡和离场单位跳过, `attacknum`随机仍消费但不限制同行实际目标数, 每个有效位置最多攻击一次.
- 回旋镖每个目标先完整执行物理攻击, 再用C `float`乘0.3并向零截断, 不补最低1点. 缩放前1至3点正伤害得到0后仍保持Normal或Critical表现, 缩放前0伤害保持Miss/Guard. 回旋镖在合击资格随机前排除且攻守任一方装备时不进入反击随机. 运行`GOCACHE="$PWD/.gocache" go test ./online -run TestCombatBoomerang -count=1`覆盖目标表、伤害边界、死亡声明目标、空排回退、攻击次数忽略、合击和反击.
- 弓的两个 `aBowW` 分支必须按声明目标列生成“同行位置后紧跟另一排同列”的十站位表, 每个站位只出现一次. 空位、死亡和离场单位不消耗 `attacknum`; 3箭只攻击顺序中前3个有效站位, 10箭最多各攻击10个有效站位一次, 击倒目标后继续扫描剩余站位. 固定随机向量必须覆盖两张精确候选表、跳空位、跳死亡、无重复目标和候选耗尽.
- 弓暴击必须保留普通暴击概率、随机次数和 `critical=true`, 但伤害只能使用普通伤害, 不得追加非弓暴击的防御与等级增伤. 弓在合击随机前排除且不消费该随机数; 角色反击使用实际武器类型的原版相性表, 弓、回旋镖、投掷斧和投掷石任一方参与时不得进入反击随机. 当前尚未实现的装备魔法、变身禁远程、守护代挡、状态抗性及其他投掷武器专用行为不得在回归说明中宣称完成.
- 防御在行动排序前生效, 并按独立动作返回.
- 合击只生成真实成员动作; 每个成员都携带独立 Damage 表现结果, 前序成员不显示伤害且没有 HP 差量, 首成员记录完整顶层来源, 最后一名成员显示并统一应用累计伤害.
- 反击链、最大深度和行动顺序稳定.
- 逃跑只移除实际参战单位, 无战宠时不得生成虚构宠物键.
- 宠物离场或死亡前已获得的战斗经验仍参与结算.
- PVE 经验持久化失败时不发布部分内存状态.
- 战斗结束后取消 timer、清理房间引用, `CombatFlowCompleteReq` 后按开关恢复自动遇敌.

## Docker 验证

1. 重新构建 online 镜像, 确保代码和 `/app/config` 来自同一工作树.
2. 启动 online 容器.
3. 查看容器日志, 确认配置加载成功、gRPC 监听成功且服务完成注册.
4. 若启动失败, 先修复日志中首个缺失文件或配置错误, 不用临时空数据绕过后续校验.
5. 发送一个未开放技能请求, 确认客户端收到业务错误且 gateway 连接保持可用.
