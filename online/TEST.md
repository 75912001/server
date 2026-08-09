# Online 服务测试指南

## 适用范围

修改 Account actor、共享配置加载、角色、宠物或邮箱业务、`CombatRoom`、回合动作、gateway stream 路由或 online gRPC handler 时使用本文档.

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
- `item.yaml` 必须使用 `items.<group>.<id>` 结构; 未知或空分组、ID超出分组区间、非法 `atlas` 路径、武器分组混用图集、随机范围倒置、非法职业或非法元素配置均应加载失败.
- 普通道具 `sprite` 为0时不能配置 `atlas`, `sprite` 大于0时必须配置以 `item/` 开头的无扩展名 `atlas`; 武器还必须使用非零 `sprite` 且不能配置 `use`.
- `pet.yaml skill` 的非 0 ID 不存在于 `skill.yaml` 时加载失败.
- `scene.yaml` 引用不存在的敌人组时加载失败.
- `enemy.group.yaml enemies[].id` 引用不存在的宠物模板时加载失败.
- 当前所有宠物技能槽都是 `[8000001,8000002,0,0,0,0,0]`.
- 配置加载失败后不注册 etcd, 不启动 gRPC 业务入口.

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
- 缺失装备快照和已装备武器都不得误触发空手多段; 合击成员与反击仍保持单段.
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
